package scanner

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxFileSize   = 512 * 1024 // 512KB - most malicious PHP files are much smaller
	chunkOverlap  = 100        // bytes of overlap between chunks
	progressEvery = 100        // send progress every N files
)

// Finding represents a detected threat in a file
type Finding struct {
	FilePath    string
	LineNumber  int
	RuleID      string
	Category    RuleCategory
	Severity    Severity
	Description string
	MatchedText string
}

// ScanStats tracks scanning progress
type ScanStats struct {
	TotalFiles   int64
	ScannedFiles int64
	ThreatsFound int64
	CurrentFile  string
}

// ScanProgress is sent to TUI for progress updates
type ScanProgress struct {
	CurrentFile  string
	ScannedFiles int64
	TotalFiles   int64
	ThreatsFound int64
	Done         bool
}

// Engine is the file scanning engine with worker pool
type Engine struct {
	rootPath    string
	filter      *ScanFilter
	matcher     *Matcher
	workerCount int
	findings    []Finding
	stats       ScanStats
	mu          sync.Mutex
	progressCh  chan ScanProgress
	throttleCh  chan struct{}
	debugLog    *log.Logger
}

// NewEngine creates a scanning engine
func NewEngine(rootPath string, mode string, includeVendor bool, progressCh chan ScanProgress, debugLog *log.Logger) *Engine {
	return &Engine{
		rootPath:    rootPath,
		filter:      NewScanFilter(mode, includeVendor),
		matcher:     NewMatcher(debugLog),
		workerCount: runtime.NumCPU() * 2,
		progressCh:  progressCh,
		debugLog:    debugLog,
	}
}

// SetThrottleChannel sets the channel used by resource limiter to pause workers
func (e *Engine) SetThrottleChannel(ch chan struct{}) {
	e.throttleCh = ch
}

// Scan starts the scanning process and returns all findings
func (e *Engine) Scan(ctx context.Context) ([]Finding, error) {
	// First pass: count total files
	totalFiles, err := e.countFiles(ctx)
	if err != nil {
		return nil, err
	}
	atomic.StoreInt64(&e.stats.TotalFiles, totalFiles)

	// Create job channel
	jobs := make(chan string, e.workerCount*4)

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < e.workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.worker(ctx, jobs)
		}()
	}

	// Second pass: walk and distribute files to workers
	err = e.walkFiles(ctx, jobs)
	close(jobs)

	// Wait for all workers to finish
	wg.Wait()

	// Send final progress
	if e.progressCh != nil {
		select {
		case e.progressCh <- ScanProgress{
			ScannedFiles: atomic.LoadInt64(&e.stats.ScannedFiles),
			TotalFiles:   atomic.LoadInt64(&e.stats.TotalFiles),
			ThreatsFound: atomic.LoadInt64(&e.stats.ThreatsFound),
			Done:         true,
		}:
		case <-ctx.Done():
		}
	}

	e.mu.Lock()
	results := make([]Finding, len(e.findings))
	copy(results, e.findings)
	e.mu.Unlock()

	return results, err
}

// GetStats returns current scan statistics
func (e *Engine) GetStats() ScanStats {
	return ScanStats{
		TotalFiles:   atomic.LoadInt64(&e.stats.TotalFiles),
		ScannedFiles: atomic.LoadInt64(&e.stats.ScannedFiles),
		ThreatsFound: atomic.LoadInt64(&e.stats.ThreatsFound),
		CurrentFile:  e.stats.CurrentFile,
	}
}

// countFiles counts the total number of scannable files
func (e *Engine) countFiles(ctx context.Context) (int64, error) {
	var count int64
	err := filepath.WalkDir(e.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			relPath, _ := filepath.Rel(e.rootPath, path)
			if relPath != "." && e.filter.ShouldSkipDir(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		if e.filter.ShouldScanFile(d.Name()) {
			count++
		}
		return nil
	})
	return count, err
}

// walkFiles walks the directory tree and sends file paths to the jobs channel
func (e *Engine) walkFiles(ctx context.Context, jobs chan<- string) error {
	return filepath.WalkDir(e.rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			relPath, _ := filepath.Rel(e.rootPath, path)
			if relPath != "." && e.filter.ShouldSkipDir(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		if e.filter.ShouldScanFile(d.Name()) {
			select {
			case jobs <- path:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})
}

// worker processes files from the jobs channel
func (e *Engine) worker(ctx context.Context, jobs <-chan string) {
	for path := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Throttle support: if throttle channel is set, check for pause signal
		if e.throttleCh != nil {
			select {
			case <-e.throttleCh:
				// Received throttle signal, back off to reduce load.
				time.Sleep(500 * time.Millisecond)
			default:
				// No throttle, continue
			}
		}

		start := time.Now()
		e.scanFile(ctx, path)
		elapsed := time.Since(start)

		if e.debugLog != nil && elapsed > 2*time.Second {
			e.debugLog.Printf("SLOW FILE: %s took %v", path, elapsed)
		}

		scanned := atomic.AddInt64(&e.stats.ScannedFiles, 1)
		if e.progressCh != nil && scanned%progressEvery == 0 {
			select {
			case e.progressCh <- ScanProgress{
				CurrentFile:  path,
				ScannedFiles: scanned,
				TotalFiles:   atomic.LoadInt64(&e.stats.TotalFiles),
				ThreatsFound: atomic.LoadInt64(&e.stats.ThreatsFound),
			}:
			case <-ctx.Done():
				return
			default:
				// Channel full, skip this progress update
			}
		}
	}
}

// scanFile reads and scans a single file
func (e *Engine) scanFile(ctx context.Context, path string) {
	// Create a per-file timeout context to prevent hanging on problematic files
	fileCtx, fileCancel := context.WithTimeout(ctx, 10*time.Second)
	defer fileCancel()

	// Open read-only
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return // skip files we can't open
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return
	}

	size := info.Size()
	if size == 0 {
		return
	}

	if size <= maxFileSize {
		// Small file: read entirely
		content, err := os.ReadFile(path)
		if err != nil {
			return
		}
		e.processMatches(fileCtx, path, content)
	} else {
		// Large file: read in chunks with overlap
		e.scanLargeFile(fileCtx, f, path, size)
	}
}

// scanLargeFile reads a file in 1MB chunks with overlap
func (e *Engine) scanLargeFile(ctx context.Context, f *os.File, path string, size int64) {
	buf := make([]byte, maxFileSize)
	var offset int64

	for offset < size {
		select {
		case <-ctx.Done():
			return
		default:
		}

		readSize := int64(maxFileSize)
		if offset+readSize > size {
			readSize = size - offset
		}

		n, err := f.ReadAt(buf[:readSize], offset)
		if n == 0 || err != nil && n == 0 {
			break
		}

		e.processMatches(ctx, path, buf[:n])

		// If this was the final chunk, stop
		if int64(n) < readSize {
			break
		}

		// Move forward by chunk size minus overlap
		offset += int64(n) - chunkOverlap
	}
}

// processMatches runs the matcher on content and records findings
func (e *Engine) processMatches(ctx context.Context, path string, content []byte) {
	matches := e.matcher.Match(ctx, content)
	if len(matches) == 0 {
		return
	}

	findings := make([]Finding, 0, len(matches))
	for _, m := range matches {
		findings = append(findings, Finding{
			FilePath:    path,
			LineNumber:  m.LineNumber,
			RuleID:      m.Rule.ID,
			Category:    m.Rule.Category,
			Severity:    m.Rule.Severity,
			Description: m.Rule.Description,
			MatchedText: m.MatchedText,
		})
	}

	atomic.AddInt64(&e.stats.ThreatsFound, int64(len(findings)))

	e.mu.Lock()
	e.findings = append(e.findings, findings...)
	e.mu.Unlock()

	// Send progress on finding
	if e.progressCh != nil {
		select {
		case e.progressCh <- ScanProgress{
			CurrentFile:  path,
			ScannedFiles: atomic.LoadInt64(&e.stats.ScannedFiles),
			TotalFiles:   atomic.LoadInt64(&e.stats.TotalFiles),
			ThreatsFound: atomic.LoadInt64(&e.stats.ThreatsFound),
		}:
		default:
			// Channel full, skip this progress update
		}
	}
}
