// Package contactout integrates with the ContactOut API.
// Docs: https://contactout.com/
//
// Endpoint: POST https://api.contactout.com/v1/domain/enrich
// Auth:     token: YOUR_KEY header
package contactout

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

const (
	enrichURL  = "https://api.contactout.com/v1/domain/enrich"
	accountURL = "https://api.contactout.com/v1/me" // placeholder/standard endpoint for check
)

// ── Account Info ──────────────────────────────────────────────────────────────

// AccountInfo holds ContactOut account details and limits.
type AccountInfo struct {
	Email        string
	Name         string
	CreditsLeft  int
	CreditsTotal int
}

type accountResp struct {
	Email        string `json:"email"`
	Name         string `json:"name"`
	CreditsLeft  int    `json:"credits_left"`
	CreditsTotal int    `json:"credits_total"`
}

// ── Search Response ────────────────────────────────────────────────────────────

type emailItem struct {
	Email string `json:"email"`
	Type  string `json:"type"`
}

type companyProfile struct {
	Emails []emailItem `json:"emails"`
}

type enrichResp struct {
	Company companyProfile   `json:"company"`
	Profiles []struct {
		Name   string      `json:"name"`
		Emails []emailItem `json:"emails"`
	} `json:"profiles"`
}

// ── HTTP helper ────────────────────────────────────────────────────────────────

func newClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func doRequest(client *http.Client, method, url, apiKey string, payload interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if payload != nil {
		b, _ := json.Marshal(payload)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("token", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.StatusCode, nil
}

// ── Public API ────────────────────────────────────────────────────────────────

// GetAccountInfo fetches ContactOut account credentials and credit status.
func GetAccountInfo(apiKey string) *AccountInfo {
	if apiKey == "" {
		return nil
	}
	red := color.New(color.FgRed)
	b, status, err := doRequest(newClient(), "GET", accountURL, apiKey, nil)
	if err != nil {
		red.Printf("  [-] ContactOut account fetch error: %v\n", err)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] ContactOut account: HTTP %d\n", status)
		return nil
	}
	var parsed accountResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] ContactOut account parse error: %v\n", err)
		return nil
	}
	return &AccountInfo{
		Email:        parsed.Email,
		Name:         parsed.Name,
		CreditsLeft:  parsed.CreditsLeft,
		CreditsTotal: parsed.CreditsTotal,
	}
}

// PrintAccountInfo displays ContactOut account details in a formatted box.
func PrintAccountInfo(info *AccountInfo) {
	cyan   := color.New(color.FgCyan, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.FgHiBlack)

	dim.Println("  ┌─ ContactOut ────────────────────────────────────────────────")
	row := func(label, val string) {
		dim.Printf("  │  ")
		cyan.Printf("%-14s", label)
		fmt.Printf(" : %s\n", val)
	}
	if info == nil {
		dim.Printf("  │  ")
		yellow.Println("API key not set — module will be skipped")
		dim.Println("  └─────────────────────────────────────────────────────────────")
		return
	}
	row("Account", info.Email+"  ("+info.Name+")")
	if info.CreditsTotal > 0 {
		bar := limitBar(info.CreditsTotal-info.CreditsLeft, info.CreditsTotal)
		row("Credits", fmt.Sprintf("%d / %d used  %s  (%d remaining)",
			info.CreditsTotal-info.CreditsLeft, info.CreditsTotal, bar, info.CreditsLeft))
	} else if info.CreditsLeft > 0 {
		row("Credits", fmt.Sprintf("%d remaining", info.CreditsLeft))
	}
	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search queries ContactOut domain enrichment endpoint.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain, apiKey string, seen *output.SeenSet) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying ContactOut API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping ContactOut — no API key provided")
		return nil
	}

	payload := map[string]string{
		"domain": domain,
	}

	b, status, err := doRequest(newClient(), "POST", enrichURL, apiKey, payload)
	if err != nil {
		red.Printf("  [-] ContactOut request error: %v\n", err)
		return nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		red.Printf("  [-] ContactOut returned HTTP %d: %s\n", status, string(b))
		return nil
	}

	var parsed enrichResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] ContactOut parse error: %v\n", err)
		return nil
	}

	var results []output.Result
	processEmail := func(email, source string) {
		email = strings.ToLower(strings.TrimSpace(email))
		if email == "" {
			return
		}
		if seen.Add(email) {
			r := output.Result{Email: email, Source: source}
			results = append(results, r)
			output.PrintResult(email, r.Source)
		}
	}

	// 1. Process company-level emails
	for _, e := range parsed.Company.Emails {
		processEmail(e.Email, "contactout.com (company)")
	}

	// 2. Process profile-level emails
	for _, p := range parsed.Profiles {
		for _, e := range p.Emails {
			processEmail(e.Email, "contactout.com (profile)")
		}
	}

	cyan.Printf("  [*] ")
	fmt.Printf("ContactOut returned %d new emails\n", len(results))

	return results
}

// ── Helper ────────────────────────────────────────────────────────────────────

func limitBar(used, total int) string {
	if total == 0 {
		return "[----------]"
	}
	filled := (used * 10) / total
	if filled > 10 {
		filled = 10
	}
	bar := "["
	for i := 0; i < 10; i++ {
		if i < filled {
			bar += "█"
		} else {
			bar += "░"
		}
	}
	bar += "]"
	pct := (used * 100) / total
	return fmt.Sprintf("%s %d%%", bar, pct)
}
