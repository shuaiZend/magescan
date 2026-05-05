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
	Exclude     *regexp.Regexp // optional: if set, match is ignored when Exclude matches
	Description string
	Severity    string
}

var dbPatterns = []dbPattern{
	{regexp.MustCompile(`(?i)<script[^>]*src\s*=\s*['"]https?://`), nil, "External script injection", "Critical"},
	{regexp.MustCompile(`(?i)eval\(`), nil, "Eval in CMS content", "Critical"},
	{regexp.MustCompile(`(?i)<iframe`), nil, "IFrame injection (possible redirect/phishing)", "High"},
	{regexp.MustCompile(`(?i)javascript:`), nil, "JavaScript protocol injection", "High"},
	{regexp.MustCompile(`(?i)document\.write\(`), nil, "Document write injection", "High"},
	{regexp.MustCompile(`(?i)base64_decode\(`), nil, "Base64 decode in CMS content", "High"},
	{regexp.MustCompile(`(?is)<script[^>]*>.*(?:atob|btoa|fetch|XMLHttpRequest)`), nil, "Suspicious inline script", "High"},
	{regexp.MustCompile(`(?i)\bonload\s*="`), nil, "Onload event handler injection", "Medium"},
	{regexp.MustCompile(`(?i)\bonerror\s*="`), nil, "Onerror event handler injection", "Medium"},
	{regexp.MustCompile(`(?i)<link[^>]*href\s*=\s*['"]https?://`), regexp.MustCompile(`(?i)(?:googleapis|gstatic|cloudflare|jquery|bootstrapcdn)`), "Suspicious external resource", "Medium"},
	{regexp.MustCompile(`(?:\.ru|\.cn|\.tk|\.pw|\.top|\.xyz|\.club|\.work|\.buzz)/`), nil, "Suspicious TLD in content", "Medium"},
	// JS Shell / Magecart patterns from Sansec threat intelligence
	{regexp.MustCompile(`(?i)<script[^>]*>\s*var\s+\w+\s*=\s*['"]`), nil, "Inline script with variable assignment (potential skimmer)", "High"},
	{regexp.MustCompile(`(?i)<script[^>]*>.*(?:createElement|appendChild|insertBefore)`), nil, "DOM manipulation in inline script", "High"},
	{regexp.MustCompile(`(?i)(?:fromCharCode|charCodeAt|String\.fromCharCode)`), nil, "Character code manipulation (JS obfuscation)", "High"},
	{regexp.MustCompile(`(?i)new\s+Image\(\).*\.src\s*=|\.src\s*=.*new\s+Image`), nil, "Image beacon exfiltration", "Critical"},
	{regexp.MustCompile(`(?i)navigator\.sendBeacon\(`), nil, "SendBeacon data exfiltration", "Critical"},
	{regexp.MustCompile(`(?i)(?:window\.)?atob\s*\(`), nil, "Base64 atob decoding in content", "High"},
	{regexp.MustCompile(`(?i)fetch\s*\(\s*['"]https?://`), nil, "Fetch to external URL", "Critical"},
	{regexp.MustCompile(`(?i)XMLHttpRequest|\.open\s*\(\s*['"]POST['"]`), nil, "XHR/POST request in content", "Critical"},
	{regexp.MustCompile(`(?i)<script[^>]*src\s*=\s*['"][^'"]*(?:cdnstatics\.net|js-csp\.com|js-stats\.com|jslibrary\.net|googletagmanager\.eu|jquerycdn\.at|cdn-sources\.com|windlrr\.com|stromao\.com|cloudflare-stat\.net)`), nil, "Known Magecart exfiltration domain (Sansec)", "Critical"},
	{regexp.MustCompile(`(?i)RTCPeerConnection|createDataChannel`), nil, "WebRTC data channel (CSP bypass skimmer)", "Critical"},
	{regexp.MustCompile(`(?i)<svg[^>]*onload\s*=`), nil, "SVG onload injection", "Critical"},
	{regexp.MustCompile(`(?i)document\.cookie.*(?:fetch|XMLHttpRequest|Image|sendBeacon)`), nil, "Cookie theft with exfiltration", "Critical"},
	{regexp.MustCompile(`(?i)localStorage\.(?:setItem|getItem)\s*\(\s*['"]_mgx_`), nil, "Magecart localStorage indicator", "Critical"},
	{regexp.MustCompile(`(?i)(?:cardnumber|securitycode|holder|expirationdate)-kao\d+`), nil, "Known Magecart form field naming (Polyovki)", "Critical"},
	{regexp.MustCompile(`(?i)querySelectorAll\s*\(\s*['"](?:input|form|select|textarea)`), nil, "Form field harvesting", "High"},
	{regexp.MustCompile(`(?i)(?:checkout|payment|billing).*(?:addEventListener|observe)`), nil, "Checkout page event monitoring", "High"},
	{regexp.MustCompile(`(?i)GTM-(?:WXN4NCG|N7PP3X2|TC8JJS2|NH2LCRH|MT3XMX7|W8FXL6X5|KQF4P5L|M9Q3LR7|M6DS7C8|55SBK75)`), nil, "Known malicious GTM container (Sansec)", "Critical"},
	{regexp.MustCompile(`(?i)(?:googleapis\.com|youtube\.com|google\.com)[^'"]*callback\s*=\s*eval`), nil, "JSONP callback eval injection", "Critical"},
	{regexp.MustCompile(`(?i)parseInt\s*\([^,]+,\s*\d+\).*String\.fromCharCode.*\^`), nil, "Base-N XOR decoding (CosmicSting)", "Critical"},
	{regexp.MustCompile(`(?i)String\.fromCharCode\.apply\s*\(\s*null\s*,\s*\[`), nil, "fromCharCode.apply array decoding", "High"},
	{regexp.MustCompile(`(?i)document\.forms.*querySelector|querySelector.*(?:cc_number|cc_cid|payment)`), nil, "Payment form field targeting", "Critical"},
	{regexp.MustCompile(`(?i)btoa\s*\(.*(?:JSON\.stringify|serialize|encodeURI)`), nil, "Data serialization with base64 encoding", "High"},
	{regexp.MustCompile(`(?i)wss?://[a-z0-9.-]+/(?:common|ws|socket)`), nil, "WebSocket C2 connection", "Critical"},
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
	"design/footer/before_body_end",
	"design/head/default",
	"design/header/default",
	"design/footer/default",
	"admin/url/custom",
	"web/default/front",
	"catalog/seo/category_url_suffix",
	"checkout/cart/crosssell_disabled",
	"dev/debug/template_hints_storefront",
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
		{"email_template", i.scanEmailTemplates},
		{"catalog_product_entity_text", i.scanProductText},
		{"layout_update", i.scanLayoutUpdates},
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
		"SELECT config_id, path, value FROM %s WHERE path IN (%s) OR path LIKE '%%script%%' OR path LIKE '%%html%%' OR path LIKE '%%design/head%%' OR path LIKE '%%design/footer%%' OR path LIKE '%%design/header%%' OR path LIKE '%%javascript%%'",
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
				if p.Exclude != nil && p.Exclude.MatchString(value.String) {
					continue
				}
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
				if p.Exclude != nil && p.Exclude.MatchString(content.String) {
					continue
				}
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
				if p.Exclude != nil && p.Exclude.MatchString(content.String) {
					continue
				}
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
				if p.Exclude != nil && p.Exclude.MatchString(comment.String) {
					continue
				}
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

func (i *Inspector) scanEmailTemplates(ctx context.Context) error {
	tableName := i.conn.TableName("email_template")
	query := fmt.Sprintf("SELECT template_id, template_code, template_text FROM %s", tableName)

	rows, err := i.conn.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var templateID int64
		var templateCode string
		var content sql.NullString

		if err := rows.Scan(&templateID, &templateCode, &content); err != nil {
			return err
		}
		scanned++

		if !content.Valid || content.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(content.String) {
				if p.Exclude != nil && p.Exclude.MatchString(content.String) {
					continue
				}
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:       tableName,
					RecordID:    templateID,
					Field:       "template_text",
					Description: p.Description,
					MatchedText: truncate(content.String, 200),
					Severity:    p.Severity,
					RemediateSQL: fmt.Sprintf(
						"-- Review email template ID %d (%s)\nUPDATE %s SET template_text = '' WHERE template_id = %d;",
						templateID, templateCode, tableName, templateID),
				})
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("email_template", scanned, threats, true)
	return nil
}

func (i *Inspector) scanProductText(ctx context.Context) error {
	tableName := i.conn.TableName("catalog_product_entity_text")
	query := fmt.Sprintf("SELECT value_id, entity_id, value FROM %s WHERE value LIKE '%%<script%%' OR value LIKE '%%javascript:%%' OR value LIKE '%%eval(%%' OR value LIKE '%%document.write%%' OR value LIKE '%%onload=%%' OR value LIKE '%%onerror=%%'", tableName)

	rows, err := i.conn.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var valueID, entityID int64
		var content sql.NullString

		if err := rows.Scan(&valueID, &entityID, &content); err != nil {
			return err
		}
		scanned++

		if !content.Valid || content.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(content.String) {
				if p.Exclude != nil && p.Exclude.MatchString(content.String) {
					continue
				}
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:       tableName,
					RecordID:    entityID,
					Field:       "value",
					Description: p.Description,
					MatchedText: truncate(content.String, 200),
					Severity:    p.Severity,
					RemediateSQL: fmt.Sprintf(
						"-- Review product entity ID %d, value_id %d\n-- SELECT * FROM %s WHERE value_id = %d;",
						entityID, valueID, tableName, valueID),
				})
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("catalog_product_entity_text", scanned, threats, true)
	return nil
}

func (i *Inspector) scanLayoutUpdates(ctx context.Context) error {
	tableName := i.conn.TableName("layout_update")
	query := fmt.Sprintf("SELECT layout_update_id, handle, xml FROM %s WHERE xml LIKE '%%<script%%' OR xml LIKE '%%javascript%%' OR xml LIKE '%%onload%%' OR xml LIKE '%%<referenceBlock%%'", tableName)

	rows, err := i.conn.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	var scanned, threats int64
	for rows.Next() {
		var updateID int64
		var handle string
		var xml sql.NullString

		if err := rows.Scan(&updateID, &handle, &xml); err != nil {
			return err
		}
		scanned++

		if !xml.Valid || xml.String == "" {
			continue
		}

		for _, p := range dbPatterns {
			if p.Pattern.MatchString(xml.String) {
				if p.Exclude != nil && p.Exclude.MatchString(xml.String) {
					continue
				}
				threats++
				i.findings = append(i.findings, DBFinding{
					Table:       tableName,
					RecordID:    updateID,
					Field:       "xml",
					Path:        handle,
					Description: p.Description,
					MatchedText: truncate(xml.String, 200),
					Severity:    p.Severity,
					RemediateSQL: fmt.Sprintf(
						"-- Review layout update ID %d (handle: %s)\nDELETE FROM %s WHERE layout_update_id = %d;",
						updateID, handle, tableName, updateID),
				})
				break
			}
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	i.sendProgress("layout_update", scanned, threats, true)
	return nil
}

// isTableNotFoundError checks if a MySQL error indicates a missing table.
func isTableNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "1146") || strings.Contains(errStr, "doesn't exist")
}
