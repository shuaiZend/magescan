package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for the TUI model
type FileProgressMsg struct {
	CurrentFile  string
	ScannedFiles int64
	TotalFiles   int64
	ThreatsFound int64
	Done         bool
}

type DBProgressMsg struct {
	Phase          string
	RecordsScanned int64
	ThreatsFound   int64
	Done           bool
}

type ScanCompleteMsg struct{}

// FileFinding is a simplified finding for display
type FileFinding struct {
	FilePath    string `json:"file_path"`
	LineNumber  int    `json:"line_number"`
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Description string `json:"description"`
	MatchedText string `json:"matched_text"`
}

// DBFindingDisplay is a simplified DB finding for display
type DBFindingDisplay struct {
	Table        string `json:"table"`
	RecordID     int64  `json:"record_id"`
	Field        string `json:"field"`
	Path         string `json:"path,omitempty"`
	Description  string `json:"description"`
	MatchedText  string `json:"matched_text"`
	Severity     string `json:"severity"`
	RemediateSQL string `json:"remediate_sql,omitempty"`
}

// Model is the main TUI model
type Model struct {
	// Progress components
	fileProgress progress.Model
	spinner      spinner.Model

	// State
	phase        string // "file_scan", "db_scan", "complete"
	currentFile  string
	scannedFiles int64
	totalFiles   int64
	fileThreats  int64
	dbPhase      string
	dbRecords    int64
	dbThreats    int64
	startTime    time.Time

	// Results (to pass to report)
	FileFindings []FileFinding
	DBFindings   []DBFindingDisplay

	// Dimensions
	width  int
	height int

	// Control
	quitting bool
	done     bool
}

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("15")).
			Background(lipgloss.Color("62")).
			Padding(0, 2)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("62")).
			Padding(0, 1)

	phaseStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("87"))

	threatStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	safeStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("82"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))
)

func NewModel() Model {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("87"))

	return Model{
		fileProgress: p,
		spinner:      s,
		phase:        "file_scan",
		startTime:    time.Now(),
		width:        80,
		height:       24,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.fileProgress.Width = msg.Width - 16
		if m.fileProgress.Width > 60 {
			m.fileProgress.Width = 60
		}
		if m.fileProgress.Width < 20 {
			m.fileProgress.Width = 20
		}
		return m, nil

	case FileProgressMsg:
		m.currentFile = msg.CurrentFile
		m.scannedFiles = msg.ScannedFiles
		m.totalFiles = msg.TotalFiles
		m.fileThreats = msg.ThreatsFound
		if msg.Done {
			m.phase = "db_scan"
		}
		return m, nil

	case DBProgressMsg:
		m.dbPhase = msg.Phase
		m.dbRecords = msg.RecordsScanned
		m.dbThreats = msg.ThreatsFound
		if msg.Done {
			m.phase = "complete"
		}
		return m, nil

	case ScanCompleteMsg:
		m.done = true
		m.phase = "complete"
		return m, tea.Quit

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case progress.FrameMsg:
		progressModel, cmd := m.fileProgress.Update(msg)
		m.fileProgress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	// Title
	title := titleStyle.Render("  MageScan - Magento 2 Security Scanner  ")
	b.WriteString("\n")
	b.WriteString(title)
	b.WriteString("\n\n")

	// File scan phase
	b.WriteString(phaseStyle.Render("  Phase: File Scanning"))
	b.WriteString("\n")

	// Progress bar
	var pct float64
	if m.totalFiles > 0 {
		pct = float64(m.scannedFiles) / float64(m.totalFiles)
	}
	progressBar := m.fileProgress.ViewAs(pct)
	pctDisplay := fmt.Sprintf(" %d%% (%d/%d)", int(pct*100), m.scannedFiles, m.totalFiles)
	b.WriteString("  ")
	b.WriteString(progressBar)
	b.WriteString(pctDisplay)
	b.WriteString("\n\n")

	// Current file
	currentFile := m.truncatePath(m.currentFile, m.width-14)
	b.WriteString(labelStyle.Render("  Current: "))
	b.WriteString(dimStyle.Render(currentFile))
	b.WriteString("\n")

	// Threats and elapsed
	elapsed := m.formatElapsed()
	threatsLabel := m.renderThreats(m.fileThreats)
	b.WriteString(fmt.Sprintf("  %s | Elapsed: %s", threatsLabel, elapsed))
	b.WriteString("\n\n")

	// DB scan phase
	if m.phase == "db_scan" || m.phase == "complete" {
		b.WriteString(phaseStyle.Render("  Phase: Database Scanning"))
		b.WriteString("\n")

		if m.phase == "db_scan" {
			b.WriteString(fmt.Sprintf("  %s Scanning %s... (%d records)",
				m.spinner.View(), m.dbPhase, m.dbRecords))
		} else {
			b.WriteString(fmt.Sprintf("  ✓ Database scan complete (%d records)", m.dbRecords))
		}
		b.WriteString("\n")

		dbThreatsLabel := m.renderThreats(m.dbThreats)
		b.WriteString(fmt.Sprintf("  %s", dbThreatsLabel))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  https://github.com/shuaiZend/magescan"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Press q to quit"))
	b.WriteString("\n")

	return b.String()
}

func (m Model) renderThreats(count int64) string {
	if count > 0 {
		return threatStyle.Render(fmt.Sprintf("Threats: %d found", count))
	}
	return safeStyle.Render("Threats: 0 found")
}

func (m Model) formatElapsed() string {
	d := time.Since(m.startTime)
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func (m Model) truncatePath(path string, maxLen int) string {
	if maxLen < 10 {
		maxLen = 10
	}
	if len(path) <= maxLen {
		return path
	}
	// Show beginning and end
	return "..." + path[len(path)-(maxLen-3):]
}
