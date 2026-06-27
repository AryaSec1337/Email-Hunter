# Email-Hunter

<div align="center">

```
███████╗███╗   ███╗ █████╗ ██╗██╗      ██╗  ██╗██╗   ██╗███╗   ██╗████████╗███████╗██████╗ 
██╔════╝████╗ ████║██╔══██╗██║██║      ██║  ██║██║   ██║████╗  ██║╚══██╔══╝██╔════╝██╔══██╗
█████╗  ██╔████╔██║███████║██║██║      ███████║██║   ██║██╔██╗ ██║   ██║   █████╗  ██████╔╝
██╔══╝  ██║╚██╔╝██║██╔══██║██║██║      ██╔══██║██║   ██║██║╚██╗██║   ██║   ██╔══╝  ██╔══██╗
███████╗██║ ╚═╝ ██║██║  ██║██║███████╗ ██║  ██║╚██████╔╝██║ ╚████║   ██║   ███████╗██║  ██║
╚══════╝╚═╝     ╚═╝╚═╝  ╚═╝╚═╝╚══════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═══╝   ╚═╝   ╚══════╝╚═╝  ╚═╝
```

**A powerful OSINT email hunting tool written in Go**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=for-the-badge)
![Platform](https://img.shields.io/badge/Platform-Linux%20|%20Windows%20|%20macOS-blue?style=for-the-badge)

</div>

---

## 🎯 Features

| Module | Description |
|--------|-------------|
| 🔑 **Hunter.io API** | Queries Hunter.io domain-search API — returns emails with confidence scores |
| 📬 **Snov.io API** | Async Snov.io domain-search — polls until complete, follows pagination |
| 📜 **crt.sh Lookup** | Enumerates subdomains via Certificate Transparency logs |
| 🔍 **Dork Search** | Queries DuckDuckGo with OSINT dorks to find exposed emails |
| 🌐 **Web Crawler** | Concurrently crawls target domain pages to extract email addresses |
| 💾 **Multi-format Output** | Save results as `.txt`, `.json`, or `.csv` |
| ⚡ **Concurrent** | Multi-goroutine crawling for fast results |
| 🎨 **Colorized Output** | Beautiful terminal output with ASCII banner |

---

## 📦 Installation

```bash
git clone https://github.com/AryaSec1337/Email-Hunter.git
cd Email-Hunter
go mod tidy
go build -o email-hunter .
```

> **Requirements:** Go 1.21+

---

---

## 🚀 Usage

```
email-hunter -d <domain> [options]
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-d <domain>` | **Target domain (required)** | — |
| `-o <file>` | Output file (`.txt` / `.json` / `.csv`) | — |
| `-p <int>` | Max pages to crawl | `50` |
| `-depth <int>` | Crawl depth | `3` |
| `-hunter-key <key>` | Hunter.io API key *(overrides config file)* | — |
| `-snov-key <key>` | Snov.io API key *(overrides config file)* | — |
| `--no-hunter` | Disable Hunter.io module | — |
| `--no-snov` | Disable Snov.io module | — |
| `--no-web` | Disable web crawler module | — |
| `--no-dork` | Disable dork search module | — |
| `--no-cert` | Disable crt.sh module | — |

### Examples

```bash
# First run — auto-creates config file, fill in keys then re-run
./email-hunter -d example.com

# Full scan using keys from config file
./email-hunter -d example.com

# Save results as JSON
./email-hunter -d example.com -o results.json

# API-only mode (fastest, no crawling needed)
./email-hunter -d example.com --no-web --no-dork --no-cert

# Free modules only — no API keys required
./email-hunter -d example.com --no-hunter --no-snov

# Override a key without editing the config file
./email-hunter -d example.com -hunter-key MY_OTHER_KEY
```

---

## 📊 Output Example

```
  # ── First run: auto-creates config file ───────────────────────────────
  [+] Config file created for the first time!
  [*] Location : /home/user/.config/.config-emailhunter
  [!] Please fill in your API keys in the config file, then re-run.

  # ── Subsequent runs: shows account info + limits ──────────────────────
  [*] Config : /home/user/.config/.config-emailhunter
  [*]   HUNTER_API_KEY:  ✔  loaded (abcd****efgh)
  [*]   SNOV_API_KEY:    ✔  loaded (1234****5678)

  ┌─ Hunter.io ─────────────────────────────────────────────────
  │  Account       : john@example.com  (John Doe)
  │  Plan          : Free
  │  Searches      : 5 / 25 used  [██░░░░░░░░] 20%  (resets 2026-07-01)
  │  Verifications : 0 / 50 used  [░░░░░░░░░░] 0%
  └─────────────────────────────────────────────────────────────

  ┌─ Snov.io ───────────────────────────────────────────────────
  │  Account       : john@example.com  (John Doe)
  │  Plan          : Starter
  │  Credits       : 150 / 500 used  [███░░░░░░░] 30%  (350 remaining)
  └─────────────────────────────────────────────────────────────

  [*] Target domain : example.com

  [*] Querying Hunter.io API...
  [+] admin@example.com                     [hunter.io (conf:95%)]
  [+] contact@example.com                   [hunter.io (conf:78%)]
  [*] Hunter.io returned 2 emails  (showing 2 / 12 total)

  [*] Querying Snov.io API...
  [*] Snov.io task started (hash: 6f15de14...)
  [+] support@example.com                   [snov.io]
  [*] Snov.io returned 1 emails  (total available: 108)

  [*] Scan complete for domain: example.com
  [*] Total unique emails found: 3
```

---

## 🏗️ Architecture

```
Email-Hunter/
├── main.go                       # Entry point, CLI flag parsing
├── internal/
│   ├── config/                   # Auto-setup + load ~/.config/.config-emailhunter
│   ├── banner/                   # ASCII art + colored terminal output
│   ├── hunterio/                 # Hunter.io domain search API
│   ├── snovio/                   # Snov.io async domain search API
│   ├── crawler/                  # Concurrent HTTP web crawler
│   ├── google/                   # DuckDuckGo dork search module
│   ├── crtsh/                    # Certificate Transparency lookup
│   └── output/                   # Results formatting & file export
└── go.mod
```

---

## ⚠️ Disclaimer

This tool is intended for **legal security research and OSINT** purposes only.  
Only use it against domains you have explicit permission to scan.  
The author is not responsible for any misuse.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">Made with ❤️ by <a href="https://github.com/AryaSec1337">AryaSec1337</a></div>
