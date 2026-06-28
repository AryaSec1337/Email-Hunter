// Package findymail integrates with the FindyMail Employee Search API.
// Docs: https://app.findymail.com/api-docs
//
// Endpoint: POST https://app.findymail.com/api/search/employees
// Auth:     Authorization: Bearer YOUR_KEY
package findymail

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
	searchURL  = "https://app.findymail.com/api/search/domain"
	accountURL = "https://app.findymail.com/api/credits"
)

// ── Account Info ──────────────────────────────────────────────────────────────

// AccountInfo holds FindyMail usage limits.
type AccountInfo struct {
	CreditsLeft  int
}

type accountResp struct {
	Credits int `json:"credits"`
}

// ── Search Response ────────────────────────────────────────────────────────────

type contact struct {
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type searchResp struct {
	Contacts []contact `json:"contacts"`
	Total    int       `json:"total"`
	Message  string    `json:"message"`
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
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
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

// GetAccountInfo fetches FindyMail credit limits.
func GetAccountInfo(apiKey string) *AccountInfo {
	if apiKey == "" {
		return nil
	}
	red := color.New(color.FgRed)
	b, status, err := doRequest(newClient(), "GET", accountURL, apiKey, nil)
	if err != nil {
		red.Printf("  [-] FindyMail account fetch error: %v\n", err)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] FindyMail account: HTTP %d\n", status)
		return nil
	}
	var parsed accountResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] FindyMail account parse error: %v\n", err)
		return nil
	}

	return &AccountInfo{
		CreditsLeft: parsed.Credits,
	}
}

// PrintAccountInfo displays FindyMail account details in a formatted box.
func PrintAccountInfo(info *AccountInfo) {
	cyan   := color.New(color.FgCyan, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.FgHiBlack)

	dim.Println("  ┌─ FindyMail ─────────────────────────────────────────────────")
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
	row("Credits", fmt.Sprintf("%d remaining", info.CreditsLeft))
	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search queries FindyMail for emails at the given domain using employee search.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain, apiKey string, seen *output.SeenSet) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying FindyMail API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping FindyMail — no API key provided")
		return nil
	}

	payload := map[string]interface{}{
		"domain": domain,
		"roles":  []string{"CEO", "Founder", "Owner", "President", "VP", "Director", "Manager", "Engineer", "Sales", "Support", "Marketing", "Admin"},
	}

	b, status, err := doRequest(newClient(), "POST", searchURL, apiKey, payload)
	if err != nil {
		red.Printf("  [-] FindyMail request error: %v\n", err)
		return nil
	}
	if status == http.StatusTooManyRequests {
		red.Println("  [-] FindyMail: rate limit hit (429) — try again later")
		return nil
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		red.Printf("  [-] FindyMail: authentication failed (HTTP %d) — check your API key\n", status)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] FindyMail returned HTTP %d: %s\n", status, string(b))
		return nil
	}

	var parsed searchResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] FindyMail parse error: %v\n", err)
		return nil
	}
	if parsed.Message != "" && len(parsed.Contacts) == 0 {
		red.Printf("  [-] FindyMail: %s\n", parsed.Message)
		return nil
	}

	var results []output.Result
	for _, c := range parsed.Contacts {
		email := strings.ToLower(c.Email)
		if email == "" {
			continue
		}
		if !seen.Add(email) {
			continue
		}
		r := output.Result{Email: email, Source: "findymail.com"}
		results = append(results, r)
		output.PrintResult(email, r.Source)
	}

	cyan.Printf("  [*] ")
	fmt.Printf("FindyMail returned %d new emails", len(results))
	if parsed.Total > 0 && parsed.Total > len(results) {
		fmt.Printf("  (total available: %d)", parsed.Total)
	}
	fmt.Println()

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
