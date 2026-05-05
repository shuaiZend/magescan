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

	// Extract DB configuration values using regex
	dbConfig := &DBConfig{
		Port: "3306", // default port
	}

	// Match 'host' => 'value'
	host := extractPHPValue(text, "host")
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

	// Match 'dbname' => 'value'
	dbConfig.DBName = extractPHPValue(text, "dbname")

	// Match 'username' => 'value'
	dbConfig.Username = extractPHPValue(text, "username")

	// Match 'password' => 'value'
	dbConfig.Password = extractPHPValue(text, "password")

	// Match 'table_prefix' => 'value'
	tablePrefix := extractPHPValue(text, "table_prefix")

	// Validate that we got at minimum a host and dbname
	if dbConfig.Host == "" && dbConfig.DBName == "" {
		return nil, "", fmt.Errorf("could not parse database configuration from env.php")
	}

	return dbConfig, tablePrefix, nil
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
