package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// ParseEnvPHP reads and parses the Magento app/etc/env.php file to extract
// database configuration and table prefix. It handles the nested PHP array
// structure: 'db' => ['connection' => ['default' => [...]]].
// Uses scope-aware parsing to avoid matching values from unrelated blocks
// (e.g., Redis session password vs DB password).
// Returns DBConfig, table_prefix, and any error encountered.
func ParseEnvPHP(filePath string) (*DBConfig, string, error) {
	// Open file read-only for safety
	f, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open env.php: %w", err)
	}
	defer f.Close()

	// Read entire file content
	info, err := f.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("failed to stat env.php: %w", err)
	}

	content := make([]byte, info.Size())
	if _, err := f.Read(content); err != nil {
		return nil, "", fmt.Errorf("failed to read env.php: %w", err)
	}

	text := string(content)

	// 1. Extract the 'db' block to scope our search
	dbBlock := extractBlock(text, "db")
	if dbBlock == "" {
		return nil, "", fmt.Errorf("could not find 'db' configuration in env.php")
	}

	// 2. Extract 'connection' => 'default' sub-block within db block
	defaultBlock := dbBlock
	connBlock := extractBlock(dbBlock, "connection")
	if connBlock != "" {
		db := extractBlock(connBlock, "default")
		if db != "" {
			defaultBlock = db
		}
	}

	// 3. Extract DB configuration values from the scoped block
	dbConfig := &DBConfig{
		Port: "3306", // default port
	}

	host := extractPHPValue(defaultBlock, "host")
	if host != "" {
		// Handle host:port format (e.g., 'localhost:3307')
		if strings.Contains(host, ":") {
			parts := strings.SplitN(host, ":", 2)
			dbConfig.Host = parts[0]
			dbConfig.Port = parts[1]
		} else {
			dbConfig.Host = host
		}
	}

	dbConfig.DBName = extractPHPValue(defaultBlock, "dbname")
	dbConfig.Username = extractPHPValue(defaultBlock, "username")
	dbConfig.Password = extractPHPValue(defaultBlock, "password")

	// 4. table_prefix is at the db block level
	tablePrefix := extractPHPValue(dbBlock, "table_prefix")

	// Validate that we got at minimum a host and dbname
	if dbConfig.Host == "" && dbConfig.DBName == "" {
		return nil, "", fmt.Errorf("could not parse database configuration from env.php")
	}

	return dbConfig, tablePrefix, nil
}

// extractBlock extracts the content of a PHP array block for a given key.
// It finds 'key' => [ and uses bracket counting to locate the matching ].
// Returns the content between the outermost [ and ] (exclusive), or empty string if not found.
func extractBlock(content, key string) string {
	pattern := regexp.MustCompile(fmt.Sprintf(`['"]%s['"]\s*=>\s*\[`, regexp.QuoteMeta(key)))
	loc := pattern.FindStringIndex(content)
	if loc == nil {
		return ""
	}

	// Find the opening '[' position (last char matched by the pattern)
	start := loc[1] - 1
	depth := 0
	for i := start; i < len(content); i++ {
		switch content[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return content[start+1 : i]
			}
		}
	}
	return ""
}

// extractPHPValue extracts a value from PHP array syntax like 'key' => 'value'.
// It handles both single-quoted and double-quoted values, as well as empty strings.
func extractPHPValue(content, key string) string {
	// Pattern matches: 'key' => 'value' or "key" => "value"
	// Also handles empty values like 'key' => ''
	pattern := fmt.Sprintf(`['"]%s['"]\s*=>\s*['"]([^'"]*)?['"]`, regexp.QuoteMeta(key))
	re := regexp.MustCompile(pattern)

	matches := re.FindStringSubmatch(content)
	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}
