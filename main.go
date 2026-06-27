package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/banner"
	"github.com/AryaSec1337/Email-Hunter/internal/config"
	"github.com/AryaSec1337/Email-Hunter/internal/crawler"
	"github.com/AryaSec1337/Email-Hunter/internal/crtsh"
	"github.com/AryaSec1337/Email-Hunter/internal/google"
	"github.com/AryaSec1337/Email-Hunter/internal/hunterio"
	"github.com/AryaSec1337/Email-Hunter/internal/output"
	"github.com/AryaSec1337/Email-Hunter/internal/snovio"
)

func main() {
	// ── CLI Flags ─────────────────────────────────────────────────────────────
	domain     := flag.String("d", "", "Target domain to hunt emails from (e.g. example.com)")
	maxPages   := flag.Int("p", 50, "Maximum number of pages to crawl")
	maxDepth   := flag.Int("depth", 3, "Maximum crawl depth")
	outFile    := flag.String("o", "", "Output file path (extension determines format: .txt .json .csv)")
	noWeb      := flag.Bool("no-web", false, "Skip web crawling")
	noDork     := flag.Bool("no-dork", false, "Skip search engine dorking")
	noCert     := flag.Bool("no-cert", false, "Skip crt.sh certificate lookup")
	noHunter   := flag.Bool("no-hunter", false, "Skip Hunter.io API")
	noSnov     := flag.Bool("no-snov", false, "Skip Snov.io API")
	hunterKey  := flag.String("hunter-key", "", "Hunter.io API key")
	snovKey    := flag.String("snov-key", "", "Snov.io API key")
	flag.Usage = usage
	flag.Parse()

	// ── Banner ────────────────────────────────────────────────────────────────
	banner.Print()

	// ── Load Config File ──────────────────────────────────────────────────────
	// Priority: CLI flag > config file > empty (module will warn)
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		color.New(color.FgYellow).Printf("  [!] Config warning: %v\n", cfgErr)
	}
	config.PrintStatus(cfg)
	fmt.Println()

	// Merge: CLI flag wins if explicitly provided, otherwise fall back to config file
	if *hunterKey == "" {
		*hunterKey = cfg.HunterAPIKey
	}
	if *snovKey == "" {
		*snovKey = cfg.SnovAPIKey
	}

	// ── Validate ──────────────────────────────────────────────────────────────
	if *domain == "" {
		color.New(color.FgRed, color.Bold).Println("  [!] Error: -d <domain> is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Normalise domain
	*domain = strings.TrimPrefix(*domain, "http://")
	*domain = strings.TrimPrefix(*domain, "https://")
	*domain = strings.TrimSuffix(*domain, "/")

	yellow := color.New(color.FgYellow, color.Bold)
	cyan   := color.New(color.FgCyan, color.Bold)

	cyan.Printf("  [*] ")
	fmt.Printf("Target domain : ")
	yellow.Println(*domain)
	fmt.Println()

	var allResults []output.Result

	// ── Module: Hunter.io API ─────────────────────────────────────────────────
	if !*noHunter {
		hunterResults := hunterio.Search(*domain, *hunterKey)
		allResults = append(allResults, hunterResults...)
		fmt.Println()
	}

	// ── Module: Snov.io API ───────────────────────────────────────────────────
	if !*noSnov {
		snovResults := snovio.Search(*domain, *snovKey)
		allResults = append(allResults, snovResults...)
		fmt.Println()
	}

	// ── Module: crt.sh ────────────────────────────────────────────────────────
	if !*noCert {
		_, certEmails := crtsh.Lookup(*domain)
		allResults = append(allResults, certEmails...)
		fmt.Println()
	}

	// ── Module: Search Engine Dorks ───────────────────────────────────────────
	if !*noDork {
		dorkResults := google.Search(*domain)
		allResults = append(allResults, dorkResults...)
		fmt.Println()
	}

	// ── Module: Web Crawler ───────────────────────────────────────────────────
	if !*noWeb {
		c := crawler.New(*domain, *maxDepth, *maxPages)
		crawlResults := c.Run()
		allResults = append(allResults, crawlResults...)
		fmt.Println()
	}

	// ── Deduplicate ───────────────────────────────────────────────────────────
	allResults = output.Deduplicate(allResults)

	// ── Summary ───────────────────────────────────────────────────────────────
	output.PrintSummary(allResults, *domain)

	if len(allResults) == 0 {
		color.New(color.FgYellow).Println("  [!] No emails found. Try different options or a different domain.")
		return
	}

	// ── Save Output ───────────────────────────────────────────────────────────
	if *outFile != "" {
		var err error
		switch {
		case strings.HasSuffix(*outFile, ".json"):
			err = output.SaveJSON(allResults, *domain, *outFile)
		case strings.HasSuffix(*outFile, ".csv"):
			err = output.SaveCSV(allResults, *outFile)
		default:
			err = output.SaveTXT(allResults, *outFile)
		}

		if err != nil {
			color.New(color.FgRed).Printf("  [!] Failed to save output: %v\n", err)
		} else {
			color.New(color.FgGreen, color.Bold).Printf("  [+] ")
			fmt.Printf("Results saved to: %s\n", *outFile)
		}
		fmt.Println()
	}
}

// usage prints a pretty help message
func usage() {
	cyan  := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)

	fmt.Println()
	cyan.Println("  USAGE:")
	fmt.Println("    email-hunter -d <domain> [options]")
	fmt.Println()
	cyan.Println("  OPTIONS:")

	flags := [][]string{
		{"-d <domain>",       "Target domain (required)"},
		{"-o <file>",         "Output file (.txt / .json / .csv)"},
		{"-p <int>",          "Max pages to crawl (default: 50)"},
		{"-depth <int>",      "Crawl depth (default: 3)"},
		{"-hunter-key <key>", "Hunter.io API key"},
		{"-snov-key <key>",   "Snov.io API key"},
		{"--no-hunter",       "Disable Hunter.io module"},
		{"--no-snov",         "Disable Snov.io module"},
		{"--no-web",          "Disable web crawler module"},
		{"--no-dork",         "Disable dork search module"},
		{"--no-cert",         "Disable crt.sh module"},
	}
	for _, f := range flags {
		green.Printf("    %-22s", f[0])
		fmt.Println(" " + f[1])
	}

	fmt.Println()
	cyan.Println("  CONFIG FILE:")
	configPath, _ := config.ConfigPath()
	green.Printf("    %-22s", "Path:")
	fmt.Println(" " + configPath)
	green.Printf("    %-22s", "Format:")
	fmt.Println(" KEY=value  (see .config-emailhunter.example)")
	green.Printf("    %-22s", "Keys:")
	fmt.Println(" HUNTER_API_KEY, SNOV_API_KEY")

	fmt.Println()
	cyan.Println("  EXAMPLES:")
	fmt.Println("    email-hunter -d example.com")
	fmt.Println("    email-hunter -d example.com -o results.json")
	fmt.Println("    email-hunter -d example.com -hunter-key YOUR_KEY  # overrides config file")
	fmt.Println("    email-hunter -d example.com --no-web --no-dork    # API-only mode")
	fmt.Println("    email-hunter -d example.com -p 100 --no-hunter --no-snov")
	fmt.Println()
}
