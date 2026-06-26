# MageScan - Magento 2 Security Scanner

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)]()
[![Release](https://img.shields.io/github/v/release/shuaiZend/magescan?style=flat)](https://github.com/shuaiZend/magescan/releases)

A high-performance, read-only security scanner for Magento 2 that detects web shells, payment skimmers, obfuscated malware, and database injections. Built in Go with a real-time TUI interface.

---

## Features

- **85+ security detection rules** across 5 threat categories (WebShell, Payment Skimmer, Obfuscation, Magento-Specific, Misc)
- **Pure read-only operation** — zero modifications to the target system
- **Real-time TUI progress** — non-scrolling terminal interface powered by Bubble Tea
- **Concurrent scan engine** — multi-worker goroutine architecture for high throughput
- **Smart file filtering** — skips vendor/, node_modules/, test/, .git/ by default
- **Database security inspection** — checks `core_config_data`, `cms_block`, `cms_page`, `sales_order_status_history`, `email_template`, `catalog_product_entity_text`, `layout_update`, `sales_order_address`, and `admin_user`
- **JSON export** — full scan results exportable for CI/CD integration
- **Context-aware cancellation** — press `q` to gracefully exit at any time
- **Resource throttling** — automatic CPU/memory limiting with hysteresis
- **Standalone binary** — no PHP dependency, deploys as a single executable

---

## Requirements

| Requirement | Details |
|-------------|---------|
| **Go 1.21+** | Required for building from source |
| **MySQL access** | Optional — needed for database scanning |
| **Target system** | Magento 2 installation (with `app/etc/env.php` and `bin/magento`) |

---

## Quick Start

No Go environment required — download and run directly from [GitHub Releases](https://github.com/shuaiZend/magescan/releases):

```bash
# Linux (amd64)
curl -sL https://github.com/shuaiZend/magescan/releases/latest/download/magescan-linux-amd64 -o magescan && chmod +x magescan && ./magescan -path /var/www/magento

# macOS (Apple Silicon)
curl -sL https://github.com/shuaiZend/magescan/releases/latest/download/magescan-darwin-arm64 -o magescan && chmod +x magescan && ./magescan -path /var/www/magento

# macOS (Intel)
curl -sL https://github.com/shuaiZend/magescan/releases/latest/download/magescan-darwin-amd64 -o magescan && chmod +x magescan && ./magescan -path /var/www/magento
```

---

## Installation

### From GitHub Releases

Download the pre-compiled binary for your platform from the [Releases page](https://github.com/shuaiZend/magescan/releases). No build tools required.

### From source

```bash
git clone https://github.com/shuaiZend/magescan.git
cd magescan
go build -o magescan ./cmd/magescan/
```

### Using go install

```bash
go install github.com/shuaiZend/magescan/cmd/magescan@latest
```

The resulting `magescan` binary is self-contained and can be copied to any target server.

---

## Usage

### Basic

```bash
# Scan a Magento installation
./magescan -path /var/www/magento

# Fast scan (default — PHP/PHTML files only)
./magescan -path /var/www/magento -mode fast

# Full scan (all suspicious file types)
./magescan -path /var/www/magento -mode full
```

### CLI Options

| Flag | Default | Description |
|------|---------|-------------|
| `-path` | `.` | Magento root path (required) |
| `-mode` | `fast` | Scan mode: `fast` (PHP/PHTML only) or `full` (all file types) |
| `-debug` | `false` | Enable debug logging to `magescan-debug.log` |
| `-output` | _(none)_ | Export full scan results to a JSON file |
| `-scan-vendor` | `false` | Include vendor/, test/, and third-party directories |
| `-cpu-limit` | `0` | Max CPU cores to use (0 = all available) |
| `-mem-limit` | `0` | Max memory in MB (0 = unlimited) |

### Examples

```bash
# Full scan with JSON export
./magescan -path /var/www/magento -mode full -output results.json

# Include vendor directory in scan
./magescan -path /var/www/magento -scan-vendor

# Debug mode for troubleshooting
./magescan -path /var/www/magento -debug

# Resource-limited scan (2 cores, 256MB cap)
./magescan -path /var/www/magento -cpu-limit 2 -mem-limit 256
```

The scanner exits with code `1` if threats are found, `0` for a clean scan.

---

## Detection Capabilities

### Web Shells / Backdoors

Detects common PHP web shells and remote code execution backdoors:

- Known shells: c99, r57, WSO, b374k, weevely, FilesMan, phpShell
- Eval-based shells (`eval($_POST[...])`, `eval(base64_decode(...))`)
- System command execution (`system()`, `exec()`, `passthru()`, `shell_exec()`, `popen()`, `proc_open()`)
- File upload backdoors and write-based persistence
- GLOBALS-based indirect function calls
- LD_PRELOAD and Visbot-specific backdoors
- GSocket backdoor and cron-based persistence
- PolyShell patterns (cookie-based MD5 auth, beacon response, polyglot files)
- Supply chain License.php backdoor activation
- C2 domain curl-pipe-php loader
- PHP code execution in session files

### Payment Skimmers / Magecart

Detects credit card theft and data exfiltration:

- Direct CC data accessors (`getCcNumber()`, `getCcCid()`)
- Data exfiltration via mail, CURL, and serialized POST
- JavaScript injection and checkout page interception
- WebSocket and WebRTC covert exfiltration channels
- Keylogger patterns (keypress/keydown event listeners)
- Known skimmer domain patterns
- SVG onload script execution and pixel injection
- XOR string decoding with known skimmer keys
- Known Magecart exfiltration domains (Sansec IOC)
- JSONP callback eval injection via Google services
- Magecart localStorage data cache and IndexedDB staging
- Service Worker intercepting payment requests

### Obfuscation Techniques

Detects code hiding and payload concealment:

- Long base64-encoded strings (>5000 chars)
- Hex-encoded variable names or strings
- `chr()` concatenation obfuscation
- String fragmentation and array-based assembly
- Variable-variable function execution (`$$var()`)
- Dynamic function name from variable
- Bitwise XOR decryption patterns
- Eval of reversed/manipulated strings
- Custom base-N encoding with XOR decryption (CosmicSting)
- `String.fromCharCode.apply` array decoding

### Magento-Specific Threats

Detects attacks targeting Magento internals:

- Core file modification and path traversal includes
- Admin credential harvesting
- Payment data logging to image files
- `.htaccess` manipulation (PHP handler for non-PHP extensions)
- Cron job backdoors
- Fake session cookies (typosquatted names)
- REST API token theft
- Direct database credential extraction
- Known malicious Google Tag Manager container IDs (Sansec)
- CMS Block injection with script (CosmicSting exploit)
- Known backdoor admin account emails (Sansec)
- Supply chain backdoor via registration.php / License.php
- SessionReaper deserialization backdoor (CVE-2025-54236)
- CosmicSting XXE/SSRF payload (CVE-2024-34102)
- Supply chain SECURE_KEY backdoor signature (Sansec 2025)

---

## Performance

| Feature | Detail |
|---------|--------|
| Concurrent workers | `2×NumCPU` goroutines for parallel file scanning |
| File filtering | Skips `vendor/`, `node_modules/`, `test/`, `tests/`, `.git/`, `update/` by default |
| Per-file timeout | 10 seconds maximum per file |
| Max file size | 512 KB — larger files are skipped |
| Long line skip | Lines exceeding 64 KB are ignored |
| Regex optimization | Non-greedy matching to prevent catastrophic backtracking |
| Memory throttling | Background monitor pauses workers when limit is exceeded, resumes at 80% |
| Graceful exit | Press `q` at any time; context cancellation propagates to all workers |

---

## Database Inspection

When `app/etc/env.php` contains valid database credentials, MageScan automatically performs read-only database checks.

### Tables Scanned

| Table | What Is Checked |
|-------|-----------------|
| `core_config_data` | Sensitive config paths and values containing scripts/HTML |
| `cms_block` | All CMS block content for injected scripts and suspicious patterns |
| `cms_page` | All CMS page content for injected scripts and suspicious patterns |
| `sales_order_status_history` | Recent order comments (last 1000) for injected content |
| `email_template` | Email templates for injected scripts and malicious content |
| `catalog_product_entity_text` | Product text attributes for hidden scripts or redirects |
| `layout_update` | Layout XML updates for injected blocks or malicious references |
| `sales_order_address` | Order address fields for template injection (CVE-2022-24086) |
| `admin_user` | Admin accounts for known backdoor emails and suspicious patterns |

### Patterns Detected (50 database signatures)

- External script injection (`<script src="...">`)
- `eval()` in CMS content
- IFrame injection
- `javascript:` protocol handlers
- `document.write()` injection
- Base64 decode / atob decoding in content
- Suspicious inline scripts (fetch, XMLHttpRequest, atob/btoa)
- DOM manipulation (`createElement`, `appendChild`, `insertBefore`)
- Event handler injection (onload, onerror)
- External resources from suspicious TLDs (.ru, .cn, .tk, .pw, .top, .xyz)
- Image beacon and sendBeacon data exfiltration
- WebRTC data channel (CSP bypass skimmer)
- SVG onload injection
- Cookie theft with exfiltration
- Magecart localStorage indicator (`_mgx_`)
- Known Magecart form field naming (Group Polyovki)
- Known Magecart exfiltration domains (Sansec IOC)
- Known malicious GTM container IDs (Sansec)
- JSONP callback eval injection
- Base-N XOR decoding (CosmicSting)
- WebSocket C2 connection
- SessionReaper deserialization payload (CVE-2025-54236)
- CosmicSting XXE/iconv payload residual (CVE-2024-34102)
- Service Worker registration in CMS content
- Known active C2 domains (2025 Sansec IOC)
- Form field harvesting and checkout page monitoring

### Remediation SQL

For each database finding, MageScan generates a ready-to-use SQL statement for manual review:

```sql
UPDATE cms_block SET content = '' WHERE block_id = 42;
```

> **Note:** The scanner is fully read-only. Generated SQL is for administrator review only.

---

## Architecture

```
magescan/
├── cmd/magescan/   # CLI entry point, flag parsing, orchestration
├── config/         # Magento root detection, env.php parsing, DB config
├── scanner/        # Concurrent scan engine, 85+ rules, pattern matcher, file filter
├── database/       # DB connector, security inspector, remediation SQL
├── resource/       # CPU/memory limiter with automatic throttling
└── ui/             # TUI progress display (Bubble Tea) and report rendering
```

---

## Supported Magento Versions

- Magento Open Source 2.0.x through 2.4.x
- Adobe Commerce (Cloud and On-Premise)
- Magento Community/Enterprise 2.x variants

Version is auto-detected from `composer.json` in the Magento root.

---

## Disclaimer

> **This tool is intended for authorized security auditing only.**
>
> Only use MageScan on systems you own or have explicit written permission to scan. Unauthorized scanning may violate applicable laws and regulations.

---

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
