package scanner

// Severity levels for malware detection rules
type Severity int

const (
	SeverityCritical Severity = iota
	SeverityHigh
	SeverityMedium
	SeverityLow
)

// String returns the human-readable severity label
func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "CRITICAL"
	case SeverityHigh:
		return "HIGH"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityLow:
		return "LOW"
	default:
		return "UNKNOWN"
	}
}

// RuleCategory classifies the type of threat detected
type RuleCategory string

const (
	CategoryWebShell    RuleCategory = "WebShell/Backdoor"
	CategorySkimmer     RuleCategory = "Payment Skimmer"
	CategoryObfuscation RuleCategory = "Obfuscation"
	CategoryMagento     RuleCategory = "Magento-Specific"
)

// Rule defines a single malware detection signature
type Rule struct {
	ID          string
	Category    RuleCategory
	Severity    Severity
	Description string
	Pattern     string // For literal string match
	Regex       string // For regex match (compiled at init)
	IsRegex     bool
}

// GetAllRules returns the complete set of malware detection signatures
func GetAllRules() []Rule {
	var rules []Rule
	rules = append(rules, getWebShellRules()...)
	rules = append(rules, getSkimmerRules()...)
	rules = append(rules, getObfuscationRules()...)
	rules = append(rules, getMagentoRules()...)
	return rules
}

// =============================================================================
// Category 1: PHP Backdoors / WebShells
// These detect common PHP web shells, backdoors, and remote code execution
// patterns frequently found in compromised Magento installations.
// =============================================================================

func getWebShellRules() []Rule {
	return []Rule{
		{
			ID: "WEBSHELL-001", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Base64 encoded eval execution",
			Pattern:     "eval(base64_decode(",
		},
		{
			ID: "WEBSHELL-002", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Compressed and base64 encoded eval (gzinflate)",
			Pattern:     "eval(gzinflate(base64_decode(",
		},
		{
			ID: "WEBSHELL-003", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Compressed and base64 encoded eval (gzuncompress)",
			Pattern:     "eval(gzuncompress(base64_decode(",
		},
		{
			ID: "WEBSHELL-004", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "ROT13 obfuscated eval execution",
			Pattern:     "eval(str_rot13(",
		},
		{
			ID: "WEBSHELL-005", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Direct eval of POST data",
			Pattern:     "eval($_POST[",
		},
		{
			ID: "WEBSHELL-006", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Direct eval of REQUEST data",
			Pattern:     "eval($_REQUEST[",
		},
		{
			ID: "WEBSHELL-007", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Direct eval of GET data",
			Pattern:     "eval($_GET[",
		},
		{
			ID: "WEBSHELL-008", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Cookie-based eval backdoor",
			Pattern:     "eval($_COOKIE[",
		},
		{
			ID: "WEBSHELL-009", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Assert backdoor via POST",
			Pattern:     "assert($_POST[",
		},
		{
			ID: "WEBSHELL-010", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Assert backdoor via REQUEST",
			Pattern:     "assert($_REQUEST[",
		},
		{
			ID: "WEBSHELL-011", Category: CategoryWebShell, Severity: SeverityHigh,
			Description: "Dynamic function creation for code execution",
			Pattern:     "create_function('',",
		},
		{
			ID: "WEBSHELL-012", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "preg_replace with /e modifier (code execution)",
			Regex:       `preg_replace\s*\(\s*['"][^'"]*?/e['"]`, IsRegex: true,
		},
		{
			ID: "WEBSHELL-013", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "GLOBALS-based indirect function call",
			Regex:       `\$GLOBALS\['[^']+'\]\(\$GLOBALS\['[^']+'\]`, IsRegex: true,
		},
		{
			ID: "WEBSHELL-014", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "System call with user-supplied input",
			Pattern:     "system($_",
		},
		{
			ID: "WEBSHELL-015", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Exec with user-supplied input",
			Pattern:     "exec($_",
		},
		{
			ID: "WEBSHELL-016", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Passthru with user-supplied input",
			Pattern:     "passthru($_",
		},
		{
			ID: "WEBSHELL-017", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Shell_exec with user-supplied input",
			Pattern:     "shell_exec($_",
		},
		{
			ID: "WEBSHELL-018", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Popen with user-supplied input",
			Pattern:     "popen($_",
		},
		{
			ID: "WEBSHELL-019", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "proc_open with user-supplied input",
			Regex:       `proc_open\s*\(.*?\$_`, IsRegex: true,
		},
		{
			ID: "WEBSHELL-020", Category: CategoryWebShell, Severity: SeverityHigh,
			Description: "File upload backdoor via copy",
			Pattern:     "copy($_FILES[",
		},
		{
			ID: "WEBSHELL-021", Category: CategoryWebShell, Severity: SeverityHigh,
			Description: "Unrestricted file upload handler",
			Pattern:     "move_uploaded_file($_FILES",
		},
		{
			ID: "WEBSHELL-022", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "File write with REQUEST data",
			Regex:       `file_put_contents\s*\(.*?\$_(REQUEST|POST|GET)`, IsRegex: true,
		},
		{
			ID: "WEBSHELL-024", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "c99shell web shell detected",
			Pattern:     "c99shell",
		},
		{
			ID: "WEBSHELL-025", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "r57shell web shell detected",
			Pattern:     "r57shell",
		},
		{
			ID: "WEBSHELL-026", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "WSO web shell detected",
			Pattern:     "wso_version",
		},
		{
			ID: "WEBSHELL-027", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "FilesMan web shell detected",
			Pattern:     "FilesMan",
		},
		{
			ID: "WEBSHELL-028", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "b374k web shell detected",
			Pattern:     "b374k",
		},
		{
			ID: "WEBSHELL-029", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Weevely backdoor shell detected",
			Pattern:     "weevely",
		},
		{
			ID: "WEBSHELL-030", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "PHPShell identifier detected",
			Pattern:     "PHPSHELL_VERSION",
		},
		{
			ID: "WEBSHELL-031", Category: CategoryWebShell, Severity: SeverityHigh,
			Description: "PHP file manager backdoor",
			Pattern:     "phpFileManager",
		},
		{
			ID: "WEBSHELL-033", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "Visbot backdoor in Magento",
			Regex:       `<\?php\s*/\*\*\*\s*Magento.*?Visbot`, IsRegex: true,
		},
		{
			ID: "WEBSHELL-034", Category: CategoryWebShell, Severity: SeverityCritical,
			Description: "LD_PRELOAD backdoor (killall)",
			Pattern:     "killall -9",
		},
	}
}

// =============================================================================
// Category 2: Payment Skimmers / Magecart
// These detect credit card skimming malware, data exfiltration, and payment
// interception patterns used in Magecart-style attacks.
// =============================================================================

func getSkimmerRules() []Rule {
	return []Rule{
		{
			ID: "SKIMMER-001", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Direct credit card number accessor",
			Pattern:     "getCcNumber()",
		},
		{
			ID: "SKIMMER-002", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "CVV data accessor",
			Pattern:     "getCcCid()",
		},
		{
			ID: "SKIMMER-003", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Credit card data exfiltration via mail",
			Regex:       `cc_number.*?mail\(|mail\(.*?cc_number`, IsRegex: true,
		},
		{
			ID: "SKIMMER-004", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Credit card data exfiltration via curl",
			Regex:       `cc_number.*?curl|curl.*?cc_number`, IsRegex: true,
		},
		{
			ID: "SKIMMER-005", Category: CategorySkimmer, Severity: SeverityHigh,
			Description: "Form data interceptor pattern",
			Regex:       `document\.forms.*?querySelector|querySelector.*?document\.forms`, IsRegex: true,
		},
		{
			ID: "SKIMMER-006", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Checkout page data interceptor",
			Regex:       `preg_match.*?onepage.*?file_put_contents`, IsRegex: true,
		},
		{
			ID: "SKIMMER-007", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Request data serialization for exfiltration",
			Pattern:     "base64_encode(serialize($_REQUEST",
		},
		{
			ID: "SKIMMER-008", Category: CategorySkimmer, Severity: SeverityHigh,
			Description: "CURL data exfiltration with POST fields",
			Pattern:     "CURLOPT_POSTFIELDS",
		},
		{
			ID: "SKIMMER-009", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Known skimmer domain patterns",
			Regex:       `https?://[a-z0-9-]+\.(top|tk|ml|ga|cf|gq|xyz|pw|cc)/[a-z0-9]+\.php`, IsRegex: true,
		},
		{
			ID: "SKIMMER-010", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "SVG onload with script execution",
			Regex:       `<svg[^>]*onload`, IsRegex: true,
		},
		{
			ID: "SKIMMER-011", Category: CategorySkimmer, Severity: SeverityHigh,
			Description: "WebSocket in PHP file (data exfiltration channel)",
			Pattern:     "new WebSocket(",
		},
		{
			ID: "SKIMMER-012", Category: CategorySkimmer, Severity: SeverityHigh,
			Description: "RTCDataChannel in PHP (covert exfiltration)",
			Pattern:     "RTCDataChannel",
		},
		{
			ID: "SKIMMER-013", Category: CategorySkimmer, Severity: SeverityHigh,
			Description: "Checkout payment interception pattern",
			Regex:       `checkout.*?payment.*?intercept|intercept.*?payment.*?checkout`, IsRegex: true,
		},
		{
			ID: "SKIMMER-014", Category: CategorySkimmer, Severity: SeverityCritical,
			Description: "Base64 encoded serialize of POST data",
			Pattern:     "base64_encode(serialize($_POST",
		},
		{
			ID: "SKIMMER-015", Category: CategorySkimmer, Severity: SeverityHigh,
			Description: "JavaScript keylogger pattern in PHP",
			Regex:       `addEventListener.*?keypress|addEventListener.*?keydown|onkeypress`, IsRegex: true,
		},
	}
}

// =============================================================================
// Category 3: Obfuscation Techniques
// These detect code obfuscation patterns used to hide malicious payloads,
// including encoding, string manipulation, and variable tricks.
// =============================================================================

func getObfuscationRules() []Rule {
	return []Rule{
		{
			ID: "OBFUSC-001", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "Extremely long base64 encoded string (>2000 chars)",
			Regex:       `[A-Za-z0-9+/=]{2000,5000}`, IsRegex: true,
		},
		{
			ID: "OBFUSC-002", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "Hex-encoded variable names or strings",
			Regex:       `\\x[0-9a-fA-F]{2}(?:\\x[0-9a-fA-F]{2}){3,50}`, IsRegex: true,
		},
		{
			ID: "OBFUSC-003", Category: CategoryObfuscation, Severity: SeverityMedium,
			Description: "String concatenation obfuscation pattern",
			Regex:       `'[a-z]{2,4}'\.'[a-z]{2,4}'\.'[a-z]{2,4}'`, IsRegex: true,
		},
		{
			ID: "OBFUSC-004", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "chr() concatenation obfuscation",
			Regex:       `chr\(\d+\)\.chr\(\d+\)(?:\.chr\(\d+\)){2,}`, IsRegex: true,
		},
		{
			ID: "OBFUSC-005", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "Variable variable function execution",
			Regex:       `\$\$\w+\s*\(`, IsRegex: true,
		},
		{
			ID: "OBFUSC-009", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "Eval of reversed/manipulated string",
			Regex:       `eval\s*\(\s*strrev\s*\(`, IsRegex: true,
		},
		{
			ID: "OBFUSC-010", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "Bitwise XOR string decryption pattern",
			Regex:       `\$\w+\s*\^\s*\$\w+.*?eval|\beval\b.*?\$\w+\s*\^\s*\$\w+`, IsRegex: true,
		},
		{
			ID: "OBFUSC-011", Category: CategoryObfuscation, Severity: SeverityMedium,
			Description: "Array-based string assembly",
			Regex:       `\$\w+\[\d+\]\.\$\w+\[\d+\]\.\$\w+\[\d+\]`, IsRegex: true,
		},
		{
			ID: "OBFUSC-012", Category: CategoryObfuscation, Severity: SeverityHigh,
			Description: "Dynamic function name from variable",
			Regex:       `\$\w+\s*=\s*['"][a-z_]+['"]\s*;\s*\$\w+\s*\(`, IsRegex: true,
		},
	}
}

// =============================================================================
// Category 4: Magento-Specific Threats
// These detect malware patterns specifically targeting Magento/Adobe Commerce
// installations, including admin credential theft and payment data theft.
// =============================================================================

func getMagentoRules() []Rule {
	return []Rule{
		{
			ID: "MAGENTO-001", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "Path traversal include to Mage.php",
			Pattern:     "include '../../../../../../app/Mage.php'",
		},
		{
			ID: "MAGENTO-002", Category: CategoryMagento, Severity: SeverityMedium,
			Description: "Mage::app() in non-standard location",
			Pattern:     "Mage::app()",
		},
		{
			ID: "MAGENTO-003", Category: CategoryMagento, Severity: SeverityCritical,
			Description: "Admin credential harvesting pattern",
			Regex:       `admin_user.*?password`, IsRegex: true,
		},
		{
			ID: "MAGENTO-004", Category: CategoryMagento, Severity: SeverityCritical,
			Description: "Payment data written to image files",
			Regex:       `fopen.*?\.(jpg|png|gif).*?(payment|cc)|(?:payment|cc).*?fopen.*?\.(jpg|png|gif)`, IsRegex: true,
		},
		{
			ID: "MAGENTO-005", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "ForceType directive for disguised PHP execution",
			Pattern:     "ForceType application/x-httpd-php",
		},
		{
			ID: "MAGENTO-006", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "Data hidden in JPEG headers with base64",
			Regex:       `JPEG-1\.1.*?base64_encode|base64_encode.*?JPEG-1\.1`, IsRegex: true,
		},
		{
			ID: "MAGENTO-007", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "Fake session cookie (typosquatted name)",
			Pattern:     `setcookie("SESSIIID"`,
		},
		{
			ID: "MAGENTO-008", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "Cron job backdoor pattern",
			Regex:       `crontab|/etc/cron\.`, IsRegex: true,
		},
		{
			ID: "MAGENTO-009", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "Modified .htaccess with PHP handler for non-PHP files",
			Regex:       `AddHandler.*?php|AddType.*?php.*?\.(jpg|png|gif|css|js)`, IsRegex: true,
		},
		{
			ID: "MAGENTO-010", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "Magento config.php credential extraction",
			Regex:       `Mage::getConfig\(\).*?decrypt|decrypt.*?password`, IsRegex: true,
		},
		{
			ID: "MAGENTO-011", Category: CategoryMagento, Severity: SeverityCritical,
			Description: "Direct database credential access",
			Regex:       `local\.xml.*?crypt|core_config_data.*?payment`, IsRegex: true,
		},
		{
			ID: "MAGENTO-012", Category: CategoryMagento, Severity: SeverityHigh,
			Description: "REST API token theft pattern",
			Regex:       `oauth_token.*?secret|admin.*?token.*?bearer`, IsRegex: true,
		},
	}
}
