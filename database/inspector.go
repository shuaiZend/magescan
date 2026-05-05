package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
)

// DBFinding represents a threat found in the database.
type DBFinding struct {
	Table        string
	RecordID     int64
	Field        string
	Path         string // For core_config_data, the config path
	Description  string
	MatchedText  string // Truncated to 200 chars
	Severity     string // "Critical", "High", "Medium", "Low"
	RemediateSQL string // SQL to fix the issue
}

// DBProgress reports database scan progress.
type DBProgress struct {
	Phase          string // "core_config_data", "cms_block", "cms_page", etc.
	RecordsScanned int64
	ThreatsFound   int64
	Done           bool
}

// dbPattern defines a malicious pattern to scan for in database content.
type dbPattern struct {
	Pattern     *regexp.Regexp
	Description string
	Severity    string
}

var dbPatterns = []dbPattern{
	{regexp.MustCompile(`(?i)<script[^>]*src\s*=\s*['"]https?://`), "External script injection", "Critical"},
	{regexp.MustCompile(`(?i)eval\(`), "Eval in CMS content", "Critical"},
	{regexp.MustCompile(`(?i)<iframe`), "IFrame injection (possible redirect/phishing)", "High"},
	{regexp.MustCompile(`(?i)javascript:`), "JavaScript protocol injection", "High"},
	{regexp.MustCompile(`(?i)document\.write\(`), "Document write injection", "High"},
	{regexp.MustCompile(`(?i)base64_decode\(`), "Base64 decode in CMS content", "High"},
	{regexp.MustCompile(`(?i)<script[^>]*>(?:(?!</script>).)*(?:atob|btoa|fetch|XMLHttpRequest)`), "Suspicious inline script", "High"},
	{regexp.MustCompile(`(?i)\bonload\s*=`), "Onload event handler injection", "Medium"},
	{regexp.MustCompile(`(?i)\bonerror\s*=`), "Onerror event handler injection", "Medium"},
	{regexp.MustCompile(`(?i)<link[^>]*href\s*=\s*['"]https?://(?!.*(?:googleapis|gstatic|cloudflare|jquery|bootstrapcdn))`), "Suspicious external resource", "Medium"},
	{regexp.MustCompile(`(?:\.ru|\.cn|\.tk|\.pw|\.top|\.xyz|\.club|\.work|\.buzz)/`), "Suspicious TLD in content", "Medium"},
}

// sensitivePaths are core_config_data paths that are commonly targeted by attackers.
var sensitivePaths = []string{
	"design/head/includes",
	"design/footer/absolute_footer",
	"design/header/welcome",
	"dev/js/session_storage_key",
	"web/cookie/cookie_domain",
	"design/head/scripts",
	"design/footer/includes",
}

// Inspector performs security scans on the Magento database.
type Inspector struct {
	conn       *Connector
	progressCh chan DBProgress
	findings   []DBFinding
}

// NewInspector creates a new database security inspector.
func NewInspector(conn *Connector, progressCh chan DBProgress) *Inspector {
	return &Inspector{
		conn:       conn,
		progressCh: progressCh,
		findings:   make([]DBFinding, 0),
	}
}

// Scan performs the full database security inspection.
func (i *Inspector) Scan(ctx context.Context) ([]DBFinding, error) {
	scanFuncs := []struct {
		name string
		fn   func(ctx context.Context) error
	}{
		{"core_config_data", i.scanCoreConfigData},
		{"cms_block", i.scanCMSBlocks},
		{"cms_page", i.scanCMSPages},
		{"sales_order_status_history", i.scanOrderStatusHistory},
	}

	for _, sf := range scanFuncs {
		select {
		case <-ctx.Done():
			return i.findings, ctx.Err()
		default:
		}

		if err := sf.fn(ctx); err != nil {
			// Table might not exist; log and continue
			if isTableNotFoundError(err) {
				i.sendProgress(sf.name, 0, 0, true)
				continue
			}
			return i.findings, fmt.Errorf("error scanning %s: %w", sf.name, err)
		}
	}

	return i.findings, nil
}

// GetFindings returns all findings.
func (i *Inspector) GetFindings() []DBFinding {
	return i.findings
}

func (i *Inspector) scanCoreConfigData(ctx context.Context) error {
	tableName := i.conn.TableName("core_config_data")

	// Build placeholders for sensitive paths
	placeholders := make([]string, len(sensitivePaths))
	args := make([]interface{}, len(sensitivePaths))
	for idx, p := range sensitivePaths {
		placeholders[idx] = "?"
		args[idx] = p
	}

	query := fmt.Sprintf(
		"SELECT config_id, path, value FROM %s WHERE path IN (%s) OR path LIKE '%%script%%' OR path LIKE '%%html%%'",
		tableName, strings.Join(placeholders, ","),
	)

	rows, err := i.conn.db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var configID int64
		var path string
		var value sql.NullString

		if err := rows.Scan(&configID, &path, &value); err != nil {
			return err
		}
		scanned++

		if !value.Valid || value.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(value.String) {
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:        tableName,
					RecordID:     configID,
					Field:        "value",
					Path:         path,
					Description:  p.Description,
					MatchedText:  truncate(value.String, 200),
					Severity:     p.Severity,
					RemediateSQL: fmt.Sprintf("UPDATE %s SET value = '' WHERE config_id = %d;", tableName, configID),
				})
				break // one finding per record
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("core_config_data", scanned, threats, true)
	return nil
}

func (i *Inspector) scanCMSBlocks(ctx context.Context) error {
	tableName := i.conn.TableName("cms_block")
	query := fmt.Sprintf("SELECT block_id, identifier, content FROM %s", tableName)

	rows, err := i.conn.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var blockID int64
		var identifier string
		var content sql.NullString

		if err := rows.Scan(&blockID, &identifier, &content); err != nil {
			return err
		}
		scanned++

		if !content.Valid || content.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(content.String) {
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:       tableName,
					RecordID:    blockID,
					Field:       "content",
					Description: p.Description,
					MatchedText: truncate(content.String, 200),
					Severity:    p.Severity,
					RemediateSQL: fmt.Sprintf(
						"-- Review and clean content for cms_block ID %d (identifier: %s)\nUPDATE %s SET content = '' WHERE block_id = %d;",
						blockID, identifier, tableName, blockID),
				})
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("cms_block", scanned, threats, true)
	return nil
}

func (i *Inspector) scanCMSPages(ctx context.Context) error {
	tableName := i.conn.TableName("cms_page")
	query := fmt.Sprintf("SELECT page_id, identifier, content FROM %s", tableName)

	rows, err := i.conn.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var pageID int64
		var identifier string
		var content sql.NullString

		if err := rows.Scan(&pageID, &identifier, &content); err != nil {
			return err
		}
		scanned++

		if !content.Valid || content.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(content.String) {
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:       tableName,
					RecordID:    pageID,
					Field:       "content",
					Description: p.Description,
					MatchedText: truncate(content.String, 200),
					Severity:    p.Severity,
					RemediateSQL: fmt.Sprintf(
						"-- Review and clean content for cms_page ID %d (identifier: %s)\nUPDATE %s SET content = '' WHERE page_id = %d;",
						pageID, identifier, tableName, pageID),
				})
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("cms_page", scanned, threats, true)
	return nil
}

func (i *Inspector) scanOrderStatusHistory(ctx context.Context) error {
	tableName := i.conn.TableName("sales_order_status_history")
	query := fmt.Sprintf("SELECT entity_id, comment FROM %s ORDER BY entity_id DESC LIMIT 1000", tableName)

	rows, err := i.conn.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var entityID int64
		var comment sql.NullString

		if err := rows.Scan(&entityID, &comment); err != nil {
			return err
		}
		scanned++

		if !comment.Valid || comment.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(comment.String) {
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:        tableName,
					RecordID:     entityID,
					Field:        "comment",
					Description:  p.Description,
					MatchedText:  truncate(comment.String, 200),
					Severity:     p.Severity,
					RemediateSQL: fmt.Sprintf("UPDATE %s SET comment = '' WHERE entity_id = %d;", tableName, entityID),
				})
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("sales_order_status_history", scanned, threats, true)
	return nil
}

func (i *Inspector) sendProgress(phase string, scanned, threats int64, done bool) {
	if i.progressCh != nil {
		i.progressCh <- DBProgress{
			Phase:          phase,
			RecordsScanned: scanned,
			ThreatsFound:   threats,
			Done:           done,
		}
	}
}

// truncate returns the first n characters of s, or s itself if shorter.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// isTableNotFoundError checks if a MySQL error indicates a missing table.
func isTableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "1146") || strings.Contains(errStr, "doesn't exist")
}
