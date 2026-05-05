# MageScan - Magento 2 Security Scanner

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS%20%7C%20Windows-lightgrey)]()

A high-performance, read-only security scanner for Magento 2 that detects web shells, payment skimmers, obfuscated malware, and database injections. 

---

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
- [Supported Magento Versions](#supported-magento-versions)
- [Detection Capabilities](#detection-capabilities)
- [Database Inspection](#database-inspection)
- [Architecture](#architecture)
- [Disclaimer](#disclaimer)
- [License](#license)

---

## Features

- **Pure read-only operation** — zero modifications to the target system
- **Dual scan modes** — Fast (PHP/PHTML only) and Full (all suspicious files)
- **70+ malware signatures** across 4 threat categories
- **Database security inspection** — scans `core_config_data`, `cms_block`, `cms_page`, and `sales_order_status_history`
- **Real-time TUI progress display** — non-scrolling terminal interface powered by Bubble Tea
- **Resource limiting** — configurable CPU and memory caps with automatic throttling
- **Automatic Magento environment detection** — validates Magento root and reads `env.php` for DB config
- **Remediation SQL generation** — produces ready-to-use SQL statements for database threats
- **Standalone binary** — no PHP dependency, deploys as a single executable

---

## Requirements

| Requirement | Details |
|-------------|---------|
| **Go 1.21+** | Required for building from source |
| **MySQL access** | Optional — needed for database scanning |
| **Target system** | Must be a Magento 2 installation (with `app/etc/env.php` and `bin/magento`) |

---

## Installation

```bash
git clone <repo-url>
cd magescan
go build -o magescan ./cmd/magescan/
```

The resulting `magescan` binary is self-contained and can be copied to any target server.

---

## Usage

### Basic Usage

```bash
# Scan current directory (must be Magento root)
./magescan

# Scan a specific Magento installation
./magescan -path /var/www/magento
```

### CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-path` | `.` | Magento root path |
| `-mode` | `fast` | Scan mode: `fast` or `full` |
| `-cpu-limit` | `0` | Max CPU cores to use (0 = all available) |
| `-mem-limit` | `0` | Max memory in MB (0 = unlimited) |
| `-output` | `terminal` | Output format: `terminal` or `json` |

### Examples

```bash
# Fast scan (PHP and PHTML files only)
./magescan -path /var/www/magento -mode fast

# Full scan (all suspicious file types)
./magescan -path /var/www/magento -mode full

# Scan with resource limits (2 CPU cores, 256MB memory cap)
./magescan -path /var/www/magento -cpu-limit 2 -mem-limit 256

# Full scan with conservative resource usage
./magescan -path /var/www/magento -mode full -cpu-limit 1 -mem-limit 128
```

### Example Output

```
MageScan v1.0.0 - Magento 2 Security Scanner
Target: /var/www/magento
Version: Magento 2.4.6
Mode: Fast Scan

═══════════════════════════════════════════════════════════
  SCAN REPORT
═══════════════════════════════════════════════════════════

  FILE THREATS (3 found)
  ──────────────────────────────────────────────────────
  [CRITICAL] WebShell/Backdoor
  File: pub/media/wysiwyg/.cache.php:1
  Rule: WEBSHELL-001 — Base64 encoded eval execution
  Match: eval(base64_decode("JGNvbm5lY3Rpb24...

  [HIGH] Payment Skimmer
  File: app/design/frontend/custom/js/checkout.js:42
  Rule: SKIMMER-015 — JavaScript keylogger pattern in PHP
  Match: addEventListener('keydown'...

  DATABASE THREATS (1 found)
  ──────────────────────────────────────────────────────
  [Critical] core_config_data (ID: 1842)
  Path: design/head/includes
  Description: External script injection
  Remediation: UPDATE core_config_data SET value = '' WHERE config_id = 1842;

═══════════════════════════════════════════════════════════
  Scan completed in 00:12 | Exit code: 1 (threats found)
═══════════════════════════════════════════════════════════
```

The scanner exits with code `1` if any threats are found, and `0` for a clean scan.

---

## Supported Magento Versions

- Magento Open Source 2.0.x through 2.4.x (all Magento 2 versions)
- Adobe Commerce (Cloud and On-Premise)
- Magento Community/Enterprise 2.x variants

Version is auto-detected from `composer.json` in the Magento root.

---

## Detection Capabilities

### Web Shells / Backdoors (34 signatures)

Detects common PHP web shells and remote code execution backdoors:

- Known shells: **c99**, **r57**, **WSO**, **b374k**, **weevely**, **FilesMan**, **phpShell**
- Custom eval-based shells (`eval($_POST[...]`, `eval(base64_decode(...)`)
- System command execution (`system()`, `exec()`, `passthru()`, `shell_exec()`, `popen()`, `proc_open()`)
- File upload backdoors and write-based persistence
- GLOBALS-based indirect function calls
- LD_PRELOAD and Visbot-specific backdoors

### Payment Skimmers / Magecart (15 signatures)

Detects credit card theft and data exfiltration:

- Direct CC data accessors (`getCcNumber()`, `getCcCid()`)
- Data exfiltration via mail, CURL, and serialized POST data
- JavaScript injection and checkout page interception
- WebSocket and WebRTC covert exfiltration channels
- Keylogger patterns (keypress/keydown event listeners)
- Known skimmer domain patterns (suspicious TLDs)
- SVG onload script execution

### Obfuscation Techniques (12 signatures)

Detects code hiding and payload concealment:

- Extremely long base64-encoded strings (>500 chars)
- `gzinflate`/`gzuncompress` chains
- `chr()` concatenation obfuscation
- String fragmentation and array-based assembly
- Hex-encoded variable names
- Variable-variable function execution (`$$var()`)
- Bitwise XOR decryption patterns
- FOPO, IonCube, and Zend Guard encoded files

### Magento-Specific Threats (12 signatures)

Detects attacks targeting Magento internals:

- Core file modification and path traversal includes
- Admin credential harvesting patterns
- Payment data logging to image files
- `.htaccess` manipulation (PHP handler for non-PHP extensions)
- Cron job backdoors
- Fake session cookies (typosquatted names)
- REST API token theft
- Direct database credential extraction

---

## Database Inspection

### Tables Scanned

| Table | What Is Checked |
|-------|-----------------|
| `core_config_data` | Sensitive config paths (`design/head/includes`, `design/footer/absolute_footer`, etc.) and any path containing "script" or "html" |
| `cms_block` | All CMS block content for injected scripts, iframes, and suspicious patterns |
| `cms_page` | All CMS page content for injected scripts, iframes, and suspicious patterns |
| `sales_order_status_history` | Recent order comments (last 1000) for injected content |

### Database Patterns Detected

- External script injection (`<script src="...">`)
- `eval()` in CMS content
- IFrame injection
- `javascript:` protocol handlers
- `document.write()` injection
- Base64 decode in content
- Suspicious inline scripts (fetch, XMLHttpRequest, atob/btoa)
- Event handler injection (onload, onerror)
- External resources from suspicious TLDs (.ru, .cn, .tk, .pw, .top, .xyz)

### Remediation SQL

For each database finding, MageScan generates a ready-to-use SQL statement:

```sql
-- Review and clean content for cms_block ID 42 (identifier: footer_links)
UPDATE cms_block SET content = '' WHERE block_id = 42;
```

> **Note:** The scanner itself is fully read-only. It connects to the database with SELECT-only queries. The generated SQL is provided for manual review and execution by the administrator.

---

## Architecture

```
magescan/
├── cmd/magescan/   # CLI entry point, flag parsing, orchestration
├── config/         # Magento root detection, env.php parsing, DB config
├── scanner/        # File scanning engine, rules, pattern matcher, file filter
├── database/       # DB connector, security inspector, remediation SQL
├── resource/       # CPU/memory limiter with automatic throttling
└── ui/             # TUI progress display (Bubble Tea) and report rendering
```

### Key Design Decisions

- **Worker pool** — Spawns `2×NumCPU` concurrent workers for file scanning
- **Chunked reading** — Large files (>1MB) are read in overlapping chunks to avoid memory spikes
- **Resource throttling** — Background monitor checks memory every 500ms; automatically pauses workers when limits are exceeded, resumes at 80% threshold (hysteresis)
- **Context cancellation** — Full support for graceful shutdown via SIGINT/SIGTERM
- **Table prefix awareness** — Respects Magento table prefixes for database queries

---

## Disclaimer

> **This tool is intended for authorized security auditing only.**
>
> Only use MageScan on systems you own or have explicit written permission to scan. Unauthorized scanning of systems may violate applicable laws and regulations.

---

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.
