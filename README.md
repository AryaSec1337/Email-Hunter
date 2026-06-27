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

### From Source

```bash
git clone https://github.com/AryaSec1337/Email-Hunter.git
cd Email-Hunter
go mod tidy
go build -o email-hunter .
```

### Requirements
- Go 1.21+

---

## 🔑 Configuration File (Recommended)

Store your API keys in a config file so you never have to pass them on the command line.

### Location

| OS | Path |
|----|------|
| Linux / macOS | `~/.config/.config-emailhunter` |
| Windows | `%USERPROFILE%\.config\.config-emailhunter` |

### Setup

```bash
# Linux / macOS
mkdir -p ~/.config
cp .config-emailhunter.example ~/.config/.config-emailhunter
nano ~/.config/.config-emailhunter
```

```powershell
# Windows (PowerShell)
New-Item -ItemType Directory -Force "$env:USERPROFILE\.config"
Copy-Item .config-emailhunter.example "$env:USERPROFILE\.config\.config-emailhunter"
notepad "$env:USERPROFILE\.config\.config-emailhunter"
```

### File Format

```ini
# Email-Hunter Configuration File
# Lines starting with '#' are comments

# Hunter.io API Key  →  https://hunter.io/api-keys
HUNTER_API_KEY=your_hunter_api_key_here

# Snov.io API Key  →  https://app.snov.io/account?settings=api
SNOV_API_KEY=your_snov_api_key_here
```

> **Priority rule:** If you also pass `-hunter-key` or `-snov-key` on the command line,  
> the CLI flag **always wins** over the config file value.

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
# Basic scan (reads API keys from ~/.config/.config-emailhunter automatically)
./email-hunter -d example.com

# Save results as JSON
./email-hunter -d example.com -o results.json

# API-only mode (fastest, no crawling)
./email-hunter -d example.com --no-web --no-dork --no-cert

# Override config file key on-the-fly
./email-hunter -d example.com -hunter-key MY_OTHER_KEY

# Free modules only (no API keys needed)
./email-hunter -d example.com --no-hunter --no-snov

# Save as CSV, skip web crawling
./email-hunter -d example.com -o emails.csv --no-web
```

---

## 📊 Output Example

```
  [*] Config file: /home/user/.config/.config-emailhunter
  [*] HUNTER_API_KEY:    loaded (abcd****efgh)
  [*] SNOV_API_KEY:      loaded (1234****5678)

  [*] Target domain : example.com

  [*] Querying Hunter.io API...
  [+] admin@example.com                     [hunter.io (conf:95%)]
  [+] contact@example.com                   [hunter.io (conf:78%)]
  [*] Hunter.io returned 2 emails

  [*] Querying Snov.io API...
  [*] Snov.io task started (hash: 6f15de14db95...)
  [+] support@example.com                   [snov.io]
  [*] Snov.io returned 1 emails

  [*] Scan complete for domain: example.com
  [*] Total unique emails found: 3
```

---

## 🏗️ Architecture

```
Email-Hunter/
├── main.go                       # Entry point, CLI flag parsing
├── .config-emailhunter.example   # Config file template
├── internal/
│   ├── config/                   # Config file loader (~/.config/.config-emailhunter)
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

This tool is intended for **legal security research and OSINT** purposes only. Only use it against domains you have explicit permission to scan. The author is not responsible for any misuse.

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">Made with ❤️ by <a href="https://github.com/AryaSec1337">AryaSec1337</a></div>
