package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/magescan/config"
	"github.com/magescan/database"
	"github.com/magescan/resource"
	"github.com/magescan/scanner"
	"github.com/magescan/ui"
)

const version = "1.0.0"

func main() {
	// Parse CLI flags
	path := flag.String("path", ".", "Magento root path")
	mode := flag.String("mode", "fast", "Scan mode: fast or full")
	cpuLimit := flag.Int("cpu-limit", 0, "Max CPU cores (0 = all)")
	memLimit := flag.Int("mem-limit", 0, "Max memory MB (0 = unlimited)")
	output := flag.String("output", "", "Export full scan results to file (JSON format)")
	debugMode := flag.Bool("debug", false, "Enable debug logging to file")
	scanVendor := flag.Bool("scan-vendor", false, "Include vendor, test, and third-party directories in scan")
	flag.Parse()

	// Set up debug logging
	var debugLog *log.Logger
	if *debugMode {
		f, err := os.Create("magescan-debug.log")
		if err == nil {
			defer f.Close()
			debugLog = log.New(f, "[DEBUG] ", log.LstdFlags|log.Lmicroseconds)
			debugLog.Println("Debug mode enabled")
		}
	}

	// Detect and validate Magento installation
	rootPath, err := config.DetectMagentoRoot(*path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Detect Magento version
	magentoVersion, err := config.DetectMagentoVersion(rootPath)
	if err != nil {
		magentoVersion = "Unknown"
	}

	// Print banner
	modeLabel := "Fast Scan"
	if *mode == "full" {
		modeLabel = "Full Scan"
	}
	fmt.Printf("MageScan v%s - Magento 2 Security Scanner\n", version)
	fmt.Printf("Target: %s\n", rootPath)
	fmt.Printf("Version: Magento %s\n", magentoVersion)
	fmt.Printf("Mode: %s\n\n", modeLabel)

	// Parse env.php for DB config
	envPath := filepath.Join(rootPath, "app", "etc", "env.php")
	dbConfig, tablePrefix, dbErr := config.ParseEnvPHP(envPath)

	// Initialize resource limiter
	limiter := resource.NewLimiter(*cpuLimit, *memLimit)
	limiter.Start()
	defer limiter.Stop()

	// Set up context with signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()

	// Create progress channels
	fileProgressCh := make(chan scanner.ScanProgress, 256)
	dbProgressCh := make(chan database.DBProgress, 256)

	// Initialize TUI
	m := ui.NewModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Track start time
	startTime := time.Now()

	// Storage for findings
	var fileFindings []scanner.Finding
	var dbFindings []database.DBFinding

	// Run scanning in a goroutine
	var scanWg sync.WaitGroup
	scanWg.Add(1)
	go func() {
		defer scanWg.Done()
		defer close(fileProgressCh)
		defer close(dbProgressCh)

		// Create and run file scanner
		engine := scanner.NewEngine(rootPath, *mode, *scanVendor, fileProgressCh, debugLog)
		engine.SetThrottleChannel(limiter.ThrottleChannel())

		findings, _ := engine.Scan(ctx)
		fileFindings = findings

		// Attempt DB scan
		if dbErr == nil && dbConfig != nil {
			conn, connErr := database.NewConnector(
				dbConfig.Host, dbConfig.Port,
				dbConfig.Username, dbConfig.Password,
				dbConfig.DBName, tablePrefix,
			)
			if connErr == nil {
				defer conn.Close()
				inspector := database.NewInspector(conn, dbProgressCh)
				dbResults, _ := inspector.Scan(ctx)
				dbFindings = dbResults
			} else {
				fmt.Fprintf(os.Stderr, "Warning: Could not connect to database: %v\n", connErr)
			}
		} else if dbErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not parse env.php for DB config: %v\n", dbErr)
		}

		// Signal scan complete
		p.Send(ui.ScanCompleteMsg{})
	}()

	// Forward file progress to TUI
	go func() {
		for prog := range fileProgressCh {
			p.Send(ui.FileProgressMsg{
				CurrentFile:  prog.CurrentFile,
				ScannedFiles: prog.ScannedFiles,
				TotalFiles:   prog.TotalFiles,
				ThreatsFound: prog.ThreatsFound,
				Done:         prog.Done,
			})
		}
	}()

	// Forward DB progress to TUI
	go func() {
		for prog := range dbProgressCh {
			p.Send(ui.DBProgressMsg{
				Phase:          prog.Phase,
				RecordsScanned: prog.RecordsScanned,
				ThreatsFound:   prog.ThreatsFound,
				Done:           prog.Done,
			})
		}
	}()

	// Run TUI (blocks until quit)
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI error: %v\n", err)
		os.Exit(1)
	}

	// Cancel context to signal scan goroutine to stop, then wait for it
	cancel()
	scanWg.Wait()

	// Convert findings to report format
	elapsed := time.Since(startTime)
	elapsedStr := fmt.Sprintf("%02d:%02d", int(elapsed.Minutes()), int(elapsed.Seconds())%60)

	var reportFileFindings []ui.FileFinding
	for _, f := range fileFindings {
		reportFileFindings = append(reportFileFindings, ui.FileFinding{
			FilePath:    f.FilePath,
			LineNumber:  f.LineNumber,
			Severity:    f.Severity.String(),
			Category:    string(f.Category),
			Description: f.Description,
			MatchedText: f.MatchedText,
		})
	}

	var reportDBFindings []ui.DBFindingDisplay
	for _, f := range dbFindings {
		reportDBFindings = append(reportDBFindings, ui.DBFindingDisplay{
			Table:        f.Table,
			RecordID:     f.RecordID,
			Field:        f.Field,
			Path:         f.Path,
			Description:  f.Description,
			MatchedText:  f.MatchedText,
			Severity:     f.Severity,
			RemediateSQL: f.RemediateSQL,
		})
	}

	// Build and render report
	reportData := ui.ReportData{
		MagentoVersion: magentoVersion,
		ScanMode:       modeLabel,
		ScanPath:       rootPath,
		TotalFiles:     int64(len(fileFindings)), // approximate from engine stats
		ElapsedTime:    elapsedStr,
		FileFindings:   reportFileFindings,
		DBFindings:     reportDBFindings,
	}

	report := ui.RenderReport(reportData)
	fmt.Println(report)

	// Export full results to file if --output specified
	if *output != "" {
		if err := ui.ExportJSON(*output, reportData); err != nil {
			fmt.Fprintf(os.Stderr, "Error exporting results: %v\n", err)
		} else {
			fmt.Printf("\nFull results exported to: %s\n", *output)
		}
	}

	// Exit code based on threats
	if len(reportFileFindings) > 0 || len(reportDBFindings) > 0 {
		os.Exit(1)
	}
}
