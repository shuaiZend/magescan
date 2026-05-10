package scanner

import (
	"path/filepath"
	"strings"
)

// ScanFilter determines which files and directories to include/exclude
type ScanFilter struct {
	Mode          string // "fast" or "full"
	IncludeVendor bool   // if true, scan vendor/test directories too
}

// Directories to always skip (regardless of flags)
// NOTE: pub/media/custom_options/ is intentionally NOT skipped - it is a critical
// PolyShell upload target (pub/media/custom_options/quote/*.php).
// Attackers upload polyglot files (image header + PHP code) to this directory.
var skipDirs = map[string]bool{
	"var/cache":             true,
	"var/page_cache":        true,
	"var/session":           true,
	"var/log":               true,
	"var/report":            true,
	"var/tmp":               true,
	"var/view_preprocessed": true,
	"var/di":                true,
	"var/generation":        true,
	"pub/static":            true,
	"generated":             true,
	".git":                  true,
}

// defaultSkipDirs are skipped unless --scan-vendor is set
var defaultSkipDirs = []string{
	"vendor",
	"node_modules",
	"test",
	"tests",
	"update",
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
func NewScanFilter(mode string, includeVendor bool) *ScanFilter {
	return &ScanFilter{Mode: mode, IncludeVendor: includeVendor}
}

// ShouldSkipDir returns true if directory should be skipped entirely
func (f *ScanFilter) ShouldSkipDir(relPath string) bool {
	// Normalize to forward slashes for consistency
	relPath = filepath.ToSlash(relPath)

	// Check exact match against always-skip dirs
	if skipDirs[relPath] {
		return true
	}

	// Check if it's a subdirectory of an always-skip directory
	for dir := range skipDirs {
		if strings.HasPrefix(relPath, dir+"/") || strings.HasPrefix(relPath+"/", dir+"/") {
			return true
		}
	}

	// Check the base name against always-skip dirs
	base := filepath.Base(relPath)
	if base == ".git" || base == "generated" {
		return true
	}

	// Unless --scan-vendor is set, skip vendor/test/node_modules/update directories
	if !f.IncludeVendor {
		for _, skip := range defaultSkipDirs {
			if base == skip {
				return true
			}
		}
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
