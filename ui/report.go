package ui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ReportData contains all data needed for the final report
type ReportData struct {
	MagentoVersion string
	ScanMode       string
	ScanPath       string
	TotalFiles     int64
	ElapsedTime    string
	FileFindings   []FileFinding
	DBFindings     []DBFindingDisplay
}

// Report styles
var (
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15"))

	criticalStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	highStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("226"))

	mediumStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("87"))

	lowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	filePathStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33")).
			Underline(true)

	sqlStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("82"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("82"))

	separatorDouble = "═══════════════════════════════════════════════════════"
	separatorSingle = "───────────────────────────────────────────────────────"
)

// RenderReport generates the final formatted report string
func RenderReport(data ReportData) string {
	var b strings.Builder

	// Header
	b.WriteString(separatorDouble + "\n")
	b.WriteString(headerStyle.Render("  MAGESCAN SECURITY REPORT") + "\n")
	b.WriteString(separatorDouble + "\n\n")

	// Target info
	b.WriteString(fmt.Sprintf("  Target:    %s\n", data.ScanPath))
	if data.MagentoVersion != "" {
		b.WriteString(fmt.Sprintf("  Version:   %s\n", data.MagentoVersion))
	}
	b.WriteString(fmt.Sprintf("  Mode:      %s\n", data.ScanMode))
	b.WriteString(fmt.Sprintf("  Duration:  %s\n", data.ElapsedTime))
	b.WriteString(fmt.Sprintf("  Files:     %d scanned\n", data.TotalFiles))

	// Summary
	b.WriteString("\n" + separatorSingle + "\n")
	b.WriteString(headerStyle.Render("  SUMMARY") + "\n")
	b.WriteString(separatorSingle + "\n")

	counts := countBySeverity(data.FileFindings, data.DBFindings)
	total := counts["CRITICAL"] + counts["HIGH"] + counts["MEDIUM"] + counts["LOW"]

	b.WriteString(fmt.Sprintf("  %s  %d\n", criticalStyle.Render("Critical:"), counts["CRITICAL"]))
	b.WriteString(fmt.Sprintf("  %s      %d\n", highStyle.Render("High:"), counts["HIGH"]))
	b.WriteString(fmt.Sprintf("  %s    %d\n", mediumStyle.Render("Medium:"), counts["MEDIUM"]))
	b.WriteString(fmt.Sprintf("  %s       %d\n", lowStyle.Render("Low:"), counts["LOW"]))

	if total > 0 {
		b.WriteString(fmt.Sprintf("  Total:     %s\n", criticalStyle.Render(fmt.Sprintf("%d threats detected", total))))
	} else {
		b.WriteString(fmt.Sprintf("  Total:     %s\n", successStyle.Render("0 threats detected")))
	}

	// If no threats, show all clear
	if total == 0 {
		b.WriteString("\n" + separatorSingle + "\n")
		b.WriteString(successStyle.Render("  ✓ All clear! No threats detected.") + "\n")
		b.WriteString(separatorDouble + "\n")
		b.WriteString(successStyle.Render("  Scan complete. No threats require attention.") + "\n")
		b.WriteString(separatorDouble + "\n")
		return b.String()
	}

	// File Threats
	if len(data.FileFindings) > 0 {
		b.WriteString("\n" + separatorSingle + "\n")
		b.WriteString(headerStyle.Render("  FILE THREATS") + "\n")
		b.WriteString(separatorSingle + "\n\n")

		sorted := sortFileFindings(data.FileFindings)
		for _, f := range sorted {
			sevLabel := renderSeverityTag(f.Severity)
			b.WriteString(fmt.Sprintf("  %s %s\n", sevLabel, f.Category))
			b.WriteString(fmt.Sprintf("  File: %s:%d\n", filePathStyle.Render(f.FilePath), f.LineNumber))
			b.WriteString(fmt.Sprintf("  Rule: %s\n", f.Description))
			matchDisplay := f.MatchedText
			if len(matchDisplay) > 80 {
				matchDisplay = matchDisplay[:77] + "..."
			}
			b.WriteString(fmt.Sprintf("  Match: %s\n\n", matchDisplay))
		}
	}

	// Database Threats
	if len(data.DBFindings) > 0 {
		b.WriteString(separatorSingle + "\n")
		b.WriteString(headerStyle.Render("  DATABASE THREATS") + "\n")
		b.WriteString(separatorSingle + "\n\n")

		for _, f := range data.DBFindings {
			sevLabel := renderSeverityTag(f.Severity)
			b.WriteString(fmt.Sprintf("  %s %s (ID: %d)\n", sevLabel, f.Table, f.RecordID))
			if f.Path != "" {
				b.WriteString(fmt.Sprintf("  Path: %s\n", f.Path))
			}
			b.WriteString(fmt.Sprintf("  Issue: %s\n", f.Description))
			matchDisplay := f.MatchedText
			if len(matchDisplay) > 80 {
				matchDisplay = matchDisplay[:77] + "..."
			}
			b.WriteString(fmt.Sprintf("  Match: %s\n", matchDisplay))
			if f.RemediateSQL != "" {
				b.WriteString(fmt.Sprintf("  \n  Fix: %s\n", sqlStyle.Render(f.RemediateSQL)))
			}
			b.WriteString("\n")
		}
	}

	// Remediation section
	sqlCommands := collectRemediationSQL(data.DBFindings)
	if len(sqlCommands) > 0 {
		b.WriteString(separatorSingle + "\n")
		b.WriteString(headerStyle.Render("  REMEDIATION") + "\n")
		b.WriteString(separatorSingle + "\n\n")
		b.WriteString("  Run the following SQL commands to clean database threats:\n\n")
		for _, sql := range sqlCommands {
			b.WriteString(fmt.Sprintf("  %s\n", sqlStyle.Render(sql)))
		}
		b.WriteString("\n")
	}

	// Footer
	b.WriteString(separatorDouble + "\n")
	b.WriteString(fmt.Sprintf("  Scan complete. %s\n", criticalStyle.Render(fmt.Sprintf("%d threats require attention.", total))))
	b.WriteString(separatorDouble + "\n")

	return b.String()
}

func renderSeverityTag(severity string) string {
	tag := fmt.Sprintf("[%s]", strings.ToUpper(severity))
	switch strings.ToUpper(severity) {
	case "CRITICAL":
		return criticalStyle.Render(tag)
	case "HIGH":
		return highStyle.Render(tag)
	case "MEDIUM":
		return mediumStyle.Render(tag)
	case "LOW":
		return lowStyle.Render(tag)
	default:
		return tag
	}
}

func countBySeverity(fileFindings []FileFinding, dbFindings []DBFindingDisplay) map[string]int {
	counts := map[string]int{
		"CRITICAL": 0,
		"HIGH":     0,
		"MEDIUM":   0,
		"LOW":      0,
	}
	for _, f := range fileFindings {
		key := strings.ToUpper(f.Severity)
		counts[key]++
	}
	for _, f := range dbFindings {
		key := strings.ToUpper(f.Severity)
		counts[key]++
	}
	return counts
}

func sortFileFindings(findings []FileFinding) []FileFinding {
	sorted := make([]FileFinding, len(findings))
	copy(sorted, findings)
	severityOrder := map[string]int{
		"CRITICAL": 0,
		"HIGH":     1,
		"MEDIUM":   2,
		"LOW":      3,
	}
	sort.Slice(sorted, func(i, j int) bool {
		oi := severityOrder[strings.ToUpper(sorted[i].Severity)]
		oj := severityOrder[strings.ToUpper(sorted[j].Severity)]
		return oi < oj
	})
	return sorted
}

func collectRemediationSQL(dbFindings []DBFindingDisplay) []string {
	var sqls []string
	for _, f := range dbFindings {
		if f.RemediateSQL != "" {
			sqls = append(sqls, f.RemediateSQL)
		}
	}
	return sqls
}
