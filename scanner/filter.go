package scanner

import (
	"path/filepath"
	"strings"
)

// ScanFilter determines which files and directories to include/exclude
type ScanFilter struct {
	Mode string // "fast" or "full"
}

// Directories to always skip
var skipDirs = map[string]bool{
	"var/cache":         true,
	"var/page_cache":    true,
	"var/session":       true,
	"var/log":           true,
	"var/report":        true,
	"var/tmp":           true,
	"pub/media/catalog": true,
	"pub/media/captcha": true,
	"pub/static":        true,
	"generated":         true,
	".git":              true,
	"node_modules":      true,
	"vendor/bin":        true,
}

// Extensions to exclude in full mode
var excludeExtsFull = map[string]bool{
	".jpg":   true,
	".jpeg":  true,
	".png":   true,
	".gif":   true,
	".svg":   true,
	".ico":   true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
	".log":   true,
	".csv":   true,
	".zip":   true,
	".tar":   true,
	".gz":    true,
	".pdf":   true,
	".css":   true,
	".map":   true,
	".lock":  true,
	".mp4":   true,
	".mp3":   true,
	".swf":   true,
}

// NewScanFilter creates a new ScanFilter with the specified mode
func NewScanFilter(mode string) *ScanFilter {
	return &ScanFilter{Mode: mode}
}

// ShouldSkipDir returns true if directory should be skipped entirely
func (f *ScanFilter) ShouldSkipDir(relPath string) bool {
	// Normalize to forward slashes for consistency
	relPath = filepath.ToSlash(relPath)

	// Check exact match
	if skipDirs[relPath] {
		return true
	}

	// Check if it's a subdirectory of a skip directory or starts with one
	for dir := range skipDirs {
		if strings.HasPrefix(relPath, dir+"/") || strings.HasPrefix(relPath+"/", dir+"/") {
			return true
		}
	}

	// Also check the base name for top-level skip dirs
	base := filepath.Base(relPath)
	if base == ".git" || base == "node_modules" || base == "generated" {
		return true
	}

	return false
}

// ShouldScanFile returns true if file should be scanned
func (f *ScanFilter) ShouldScanFile(fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))

	if f.Mode == "fast" {
		return ext == ".php" || ext == ".phtml"
	}

	// Full mode: scan everything except excluded extensions
	return !excludeExtsFull[ext]
}
