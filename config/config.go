// Package config provides configuration structures and detection utilities
// for the Magento 2 security scanner.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// ScanConfig holds all configuration for a scan session.
type ScanConfig struct {
	Path        string   // Magento root path
	Mode        string   // "fast" or "full"
	CPULimit    int      // Max CPU cores to use
	MemLimit    int      // Max memory in MB
	Output      string   // "terminal" or "json"
	DBConfig    DBConfig // Database configuration
	MagentoVer  string   // Detected Magento version
	TablePrefix string   // DB table prefix
}

// DBConfig holds database connection parameters.
type DBConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	DBName   string
}

// NewDefaultConfig returns a ScanConfig with sensible defaults.
func NewDefaultConfig() *ScanConfig {
	return &ScanConfig{
		Path:     ".",
		Mode:     "fast",
		CPULimit: runtime.NumCPU(),
		MemLimit: 512,
		Output:   "terminal",
		DBConfig: DBConfig{
			Host: "localhost",
			Port: "3306",
		},
	}
}

// DetectMagentoRoot verifies that the given path is a valid Magento 2 root
// by checking for the existence of app/etc/env.php and bin/magento.
// Returns the absolute path to the Magento root or an error.
func DetectMagentoRoot(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check for app/etc/env.php
	envPHP := filepath.Join(absPath, "app", "etc", "env.php")
	if _, err := os.Stat(envPHP); os.IsNotExist(err) {
		return "", fmt.Errorf("not a Magento root: missing app/etc/env.php at %s", absPath)
	}

	// Check for bin/magento
	binMagento := filepath.Join(absPath, "bin", "magento")
	if _, err := os.Stat(binMagento); os.IsNotExist(err) {
		return "", fmt.Errorf("not a Magento root: missing bin/magento at %s", absPath)
	}

	return absPath, nil
}

// composerJSON is a minimal representation of Magento's composer.json
// used for version detection.
type composerJSON struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

// DetectMagentoVersion reads composer.json from the Magento root and
// extracts the version string.
func DetectMagentoVersion(rootPath string) (string, error) {
	composerPath := filepath.Join(rootPath, "composer.json")

	// Open file read-only for safety
	f, err := os.OpenFile(composerPath, os.O_RDONLY, 0)
	if err != nil {
		return "", fmt.Errorf("failed to open composer.json: %w", err)
	}
	defer f.Close()

	var data composerJSON
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&data); err != nil {
		return "", fmt.Errorf("failed to parse composer.json: %w", err)
	}

	if data.Version == "" {
		// Try to infer from package name
		if data.Name == "magento/magento2ce" || data.Name == "magento/magento2ee" {
			return "2.x (version not specified in composer.json)", nil
		}
		return "", fmt.Errorf("version not found in composer.json")
	}

	return data.Version, nil
}
