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
	"github.com/AryaSec1337/Email-Hunter/internal/rocketreach"
	"github.com/AryaSec1337/Email-Hunter/internal/prospeo"
	"github.com/AryaSec1337/Email-Hunter/internal/findymail"
	"github.com/AryaSec1337/Email-Hunter/internal/contactout"
	"github.com/AryaSec1337/Email-Hunter/internal/update"
)

const currentVersion = "v1.2.0"

func main() {
	// ── CLI Flags ─────────────────────────────────────────────────────────────
	domain          := flag.String("d", "", "Target domain to hunt emails from (e.g. example.com)")
	maxPages        := flag.Int("p", 50, "Maximum number of pages to crawl")
	maxDepth        := flag.Int("depth", 3, "Maximum crawl depth")
	outFile         := flag.String("o", "", "Output file path (extension determines format: .txt .json .csv)")
	updateFlag      := flag.Bool("update", false, "Update Email-Hunter to the latest version")
	
	// Skip flags
	noWeb           := flag.Bool("no-web", false, "Skip web crawling")
	noDork          := flag.Bool("no-dork", false, "Skip search engine dorking")
	noCert          := flag.Bool("no-cert", false, "Skip crt.sh certificate lookup")
	noHunter        := flag.Bool("no-hunter", false, "Skip Hunter.io API")
	noSnov          := flag.Bool("no-snov", false, "Skip Snov.io API")
	noRocketReach   := flag.Bool("no-rocketreach", false, "Skip RocketReach API")
	noProspeo       := flag.Bool("no-prospeo", false, "Skip Prospeo API")
	noFindyMail     := flag.Bool("no-findymail", false, "Skip FindyMail API")
	noContactOut    := flag.Bool("no-contactout", false, "Skip ContactOut API")

	// API Keys (override config file)
	hunterKey       := flag.String("hunter-key", "", "Hunter.io API key")
	snovID          := flag.String("snov-id", "", "Snov.io API User ID")
	snovSecret      := flag.String("snov-secret", "", "Snov.io API Secret")
	rocketreachKey  := flag.String("rocketreach-key", "", "RocketReach API key")
	prospeoKey      := flag.String("prospeo-key", "", "Prospeo API key")
	findymailKey    := flag.String("findymail-key", "", "FindyMail API key")
	contactoutKey   := flag.String("contactout-key", "", "ContactOut API key")

	flag.Usage = usage
	flag.Parse()

	// ── Handle Self-Update Flag ───────────────────────────────────────────────
	if *updateFlag {
		banner.Print()
		if err := update.UpdateSelf(); err != nil {
			color.New(color.FgRed, color.Bold).Printf("  [-] Update error: %v\n\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	// ── Banner ────────────────────────────────────────────────────────────────
	banner.Print()

	// ── Auto-setup Config File ────────────────────────────────────────────────
	cyan   := color.New(color.FgCyan, color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow, color.Bold)
	red    := color.New(color.FgRed, color.Bold)

	result, cfgPath, setupErr := config.Setup()
	switch result {
	case config.SetupCreated:
		green.Printf("  [+] ")
		fmt.Println("Config file created for the first time!")
		cyan.Printf("  [*] ")
		fmt.Printf("Location : %s\n", cfgPath)
		yellow.Println("  [!] Please fill in your API keys in the config file, then re-run.")
		yellow.Printf("  [!] File: %s\n\n", cfgPath)

	case config.SetupFailed:
		yellow.Printf("  [!] Could not create config file: %v\n\n", setupErr)

	case config.SetupAlreadyExists:
		// Normal run
	}

	// ── Load Config Values ────────────────────────────────────────────────────
	cfg, cfgErr := config.Load()
	if cfgErr != nil {
		yellow.Printf("  [!] Config load warning: %v\n", cfgErr)
	}

	// Merge keys: CLI flag wins, falls back to config file
	if *hunterKey == "" {
		*hunterKey = cfg.HunterAPIKey
	}
	if *snovID == "" {
		*snovID = cfg.SnovUserID
	}
	if *snovSecret == "" {
		*snovSecret = cfg.SnovAPISecret
	}
	if *rocketreachKey == "" {
		*rocketreachKey = cfg.RocketReachAPIKey
	}
	if *prospeoKey == "" {
		*prospeoKey = cfg.ProspeoAPIKey
	}
	if *findymailKey == "" {
		*findymailKey = cfg.FindyMailAPIKey
	}
	if *contactoutKey == "" {
		*contactoutKey = cfg.ContactOutAPIKey
	}

	// Reflect merged state back to config struct
	cfg.HunterAPIKey = *hunterKey
	cfg.SnovUserID = *snovID
	cfg.SnovAPISecret = *snovSecret
	cfg.RocketReachAPIKey = *rocketreachKey
	cfg.ProspeoAPIKey = *prospeoKey
	cfg.FindyMailAPIKey = *findymailKey
	cfg.ContactOutAPIKey = *contactoutKey

	// ── Print Config Status ───────────────────────────────────────────────────
	config.PrintStatus(cfg, *noHunter, *noSnov, *noRocketReach, *noProspeo, *noFindyMail, *noContactOut)
	fmt.Println()

	// ── Fetch & Display Account Info + Limits ─────────────────────────────────
	var hunterInfo *hunterio.AccountInfo
	var snovInfo *snovio.AccountInfo
	var rocketreachInfo *rocketreach.AccountInfo
	var prospeoInfo *prospeo.AccountInfo
	var findymailInfo *findymail.AccountInfo
	var contactoutInfo *contactout.AccountInfo

	if *hunterKey != "" {
		hunterInfo = hunterio.GetAccountInfo(*hunterKey)
	}
	if *snovID != "" && *snovSecret != "" {
		snovInfo = snovio.GetAccountInfo(*snovID, *snovSecret)
	}
	if *rocketreachKey != "" {
		rocketreachInfo = rocketreach.GetAccountInfo(*rocketreachKey)
	}
	if *prospeoKey != "" {
		prospeoInfo = prospeo.GetAccountInfo(*prospeoKey)
	}
	if *findymailKey != "" {
		findymailInfo = findymail.GetAccountInfo(*findymailKey)
	}
	if *contactoutKey != "" {
		contactoutInfo = contactout.GetAccountInfo(*contactoutKey)
	}

	hunterio.PrintAccountInfo(hunterInfo, *noHunter)
	snovio.PrintAccountInfo(snovInfo, *noSnov)
	rocketreach.PrintAccountInfo(rocketreachInfo, *noRocketReach)
	prospeo.PrintAccountInfo(prospeoInfo, *noProspeo)
	findymail.PrintAccountInfo(findymailInfo, *noFindyMail)
	contactout.PrintAccountInfo(contactoutInfo, *noContactOut)
	fmt.Println()

	// ── Warn About Missing API Keys ───────────────────────────────────────────
	config.WarnMissingKeys(cfg, *noHunter, *noSnov, *noRocketReach, *noProspeo, *noFindyMail, *noContactOut)

	// ── Check Version Update ──────────────────────────────────────────────────
	if latest, hasUpdate := update.CheckLatest(currentVersion); hasUpdate {
		yellow.Printf("  [!] A new version of Email-Hunter is available: %s (Current: %s)\n", latest, currentVersion)
		color.New(color.FgHiBlack).Println("      Run 'email-hunter -update' to update automatically.")
		fmt.Println()
	}

	// ── Validate Domain ───────────────────────────────────────────────────────
	if *domain == "" {
		red.Println("  [!] Error: -d <domain> is required\n")
		flag.Usage()
		os.Exit(1)
	}

	// Normalise domain
	*domain = strings.TrimPrefix(*domain, "http://")
	*domain = strings.TrimPrefix(*domain, "https://")
	*domain = strings.TrimSuffix(*domain, "/")

	cyan.Printf("  [*] ")
	fmt.Printf("Target domain : ")
	yellow.Println(*domain)
	fmt.Println()

	var allResults []output.Result

	// ── Global deduplication set ──────────────────────────────────────────────
	seen := output.NewSeenSet()

	// ── Module: Hunter.io API ─────────────────────────────────────────────────
	if !*noHunter {
		hunterResults := hunterio.Search(*domain, *hunterKey, seen)
		allResults = append(allResults, hunterResults...)
		fmt.Println()
	}

	// ── Module: Snov.io API ───────────────────────────────────────────────────
	if !*noSnov {
		snovResults := snovio.Search(*domain, *snovID, *snovSecret, seen)
		allResults = append(allResults, snovResults...)
		fmt.Println()
	}

	// ── Module: RocketReach API ───────────────────────────────────────────────
	if !*noRocketReach {
		rocketreachResults := rocketreach.Search(*domain, *rocketreachKey, seen)
		allResults = append(allResults, rocketreachResults...)
		fmt.Println()
	}

	// ── Module: Prospeo API ───────────────────────────────────────────────────
	if !*noProspeo {
		prospeoResults := prospeo.Search(*domain, *prospeoKey, seen)
		allResults = append(allResults, prospeoResults...)
		fmt.Println()
	}

	// ── Module: FindyMail API ─────────────────────────────────────────────────
	if !*noFindyMail {
		findymailResults := findymail.Search(*domain, *findymailKey, seen)
		allResults = append(allResults, findymailResults...)
		fmt.Println()
	}

	// ── Module: ContactOut API ────────────────────────────────────────────────
	if !*noContactOut {
		contactoutResults := contactout.Search(*domain, *contactoutKey, seen)
		allResults = append(allResults, contactoutResults...)
		fmt.Println()
	}

	// ── Module: crt.sh ────────────────────────────────────────────────────────
	if !*noCert {
		_, certEmails := crtsh.Lookup(*domain, seen)
		allResults = append(allResults, certEmails...)
		fmt.Println()
	}

	// ── Module: Search Engine Dorks ───────────────────────────────────────────
	if !*noDork {
		dorkResults := google.Search(*domain, seen)
		allResults = append(allResults, dorkResults...)
		fmt.Println()
	}

	// ── Module: Web Crawler ───────────────────────────────────────────────────
	if !*noWeb {
		c := crawler.New(*domain, *maxDepth, *maxPages)
		crawlResults := c.Run(seen)
		allResults = append(allResults, crawlResults...)
		fmt.Println()
	}

	// ── Summary ───────────────────────────────────────────────────────────────
	output.PrintSummary(allResults, *domain)

	if len(allResults) == 0 {
		yellow.Println("  [!] No emails found. Try different options or a different domain.")
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
			red.Printf("  [!] Failed to save output: %v\n", err)
		} else {
			green.Printf("  [+] ")
			fmt.Printf("Results saved to: %s\n", *outFile)
		}
		fmt.Println()
	}
}

// usage prints a pretty help message.
func usage() {
	cyan  := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)

	fmt.Println()
	cyan.Println("  USAGE:")
	fmt.Println("    email-hunter -d <domain> [options]")
	fmt.Println()
	cyan.Println("  OPTIONS:")

	flags := [][]string{
		{"-d <domain>",          "Target domain (required)"},
		{"-o <file>",            "Output file (.txt / .json / .csv)"},
		{"-p <int>",             "Max pages to crawl (default: 50)"},
		{"-depth <int>",         "Crawl depth (default: 3)"},
		{"-update",              "Update Email-Hunter to the latest version"},
		{"-hunter-key <key>",    "Hunter.io API key (overrides config file)"},
		{"-snov-id <id>",        "Snov.io API User ID (overrides config file)"},
		{"-snov-secret <sec>",   "Snov.io API Secret (overrides config file)"},
		{"-rocketreach-key <key>", "RocketReach API key (overrides config file)"},
		{"-prospeo-key <key>",   "Prospeo API key (overrides config file)"},
		{"-findymail-key <key>", "FindyMail API key (overrides config file)"},
		{"-contactout-key <k>",  "ContactOut API key (overrides config file)"},
		{"--no-hunter",          "Disable Hunter.io module"},
		{"--no-snov",            "Disable Snov.io module"},
		{"--no-rocketreach",     "Disable RocketReach module"},
		{"--no-prospeo",         "Disable Prospeo module"},
		{"--no-findymail",       "Disable FindyMail module"},
		{"--no-contactout",      "Disable ContactOut module"},
		{"--no-web",             "Disable web crawler module"},
		{"--no-dork",            "Disable dork search module"},
		{"--no-cert",            "Disable crt.sh module"},
	}
	for _, f := range flags {
		green.Printf("    %-24s", f[0])
		fmt.Println(" " + f[1])
	}

	fmt.Println()
	cyan.Println("  CONFIG FILE:")
	cfgPath, _ := config.ConfigPath()
	green.Printf("    %-24s", "Path:")
	fmt.Println(" " + cfgPath)
	green.Printf("    %-24s", "Auto-created:")
	fmt.Println(" yes — generated on first run if missing")
	green.Printf("    %-24s", "Keys:")
	fmt.Println(" HUNTER_API_KEY, SNOV_USER_ID, SNOV_API_SECRET, ROCKETREACH_API_KEY, PROSPEO_API_KEY, FINDYMAIL_API_KEY, CONTACTOUT_API_KEY")

	fmt.Println()
	cyan.Println("  EXAMPLES:")
	fmt.Println("    email-hunter -d example.com")
	fmt.Println("    email-hunter -d example.com -o results.json")
	fmt.Println("    email-hunter -d example.com --no-web --no-dork    # API-only, fastest")
	fmt.Println("    email-hunter -d example.com --no-hunter --no-snov # free modules only")
	fmt.Println()
}
