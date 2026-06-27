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
| 🌐 **Web Crawler** | Concurrently crawls target domain pages to extract email addresses |
| 🔍 **Dork Search** | Queries DuckDuckGo with OSINT dorks to find exposed emails |
| 📜 **crt.sh Lookup** | Enumerates subdomains via Certificate Transparency logs |
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
| `--no-web` | Disable web crawler module | — |
| `--no-dork` | Disable dork search module | — |
| `--no-cert` | Disable crt.sh module | — |

### Examples

```bash
# Basic scan
./email-hunter -d example.com

# Save results as JSON
./email-hunter -d example.com -o results.json

# Crawl more pages, skip dork search
./email-hunter -d example.com -p 100 --no-dork

# Save as CSV, skip web crawling
./email-hunter -d example.com -o emails.csv --no-web

# Only use crt.sh (fastest)
./email-hunter -d example.com --no-web --no-dork
```

---

## 📊 Output Example

```
[+] admin@example.com             [web-crawl]
[+] contact@example.com           [dork-search]
[+] support@mail.example.com      [crt.sh]

[*] Scan complete for domain: example.com
[*] Total unique emails found: 3
```

---

## 🏗️ Architecture

```
Email-Hunter/
├── main.go                  # Entry point, CLI flag parsing
├── internal/
│   ├── banner/              # ASCII art + colored terminal output
│   ├── crawler/             # Concurrent HTTP web crawler
│   ├── google/              # DuckDuckGo dork search module
│   ├── crtsh/               # Certificate Transparency lookup
│   └── output/              # Results formatting & file export
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
