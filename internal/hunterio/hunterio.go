// Package hunterio integrates with the Hunter.io Domain Search API.
// Docs: https://hunter.io/api-documentation/v2#domain-search
package hunterio

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

// ── Account Info Types ────────────────────────────────────────────────────────

// AccountInfo holds Hunter.io account details and usage limits.
type AccountInfo struct {
	Email      string
	FirstName  string
	LastName   string
	PlanName   string
	ResetDate  string
	SearchUsed int
	SearchMax  int
	VerifUsed  int
	VerifMax   int
}

type accountResponse struct {
	Data struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Email     string `json:"email"`
		PlanName  string `json:"plan_name"`
		ResetDate string `json:"reset_date"`
		Requests  struct {
			Searches struct {
				Used      int `json:"used"`
				Available int `json:"available"`
			} `json:"searches"`
			Verifications struct {
				Used      int `json:"used"`
				Available int `json:"available"`
			} `json:"verifications"`
		} `json:"requests"`
	} `json:"data"`
}

// ── Search Response Types ─────────────────────────────────────────────────────

type emailEntry struct {
	Value      string `json:"value"`
	Type       string `json:"type"`
	Confidence int    `json:"confidence"`
	FirstName  string `json:"first_name"`
	LastName   string `json:"last_name"`
	Position   string `json:"position"`
}

type apiResponse struct {
	Data struct {
		Domain string       `json:"domain"`
		Emails []emailEntry `json:"emails"`
	} `json:"data"`
	Meta struct {
		Results int `json:"results"`
		Limit   int `json:"limit"`
		Offset  int `json:"offset"`
	} `json:"meta"`
	Errors []struct {
		ID      string `json:"id"`
		Code    int    `json:"code"`
		Details string `json:"details"`
	} `json:"errors"`
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func newClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

func doGet(client *http.Client, rawURL string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", "EmailHunter/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// ── Public API ────────────────────────────────────────────────────────────────

// GetAccountInfo fetches account credentials and usage limits from Hunter.io.
func GetAccountInfo(apiKey string) *AccountInfo {
	if apiKey == "" {
		return nil
	}

	red := color.New(color.FgRed)
	apiURL := fmt.Sprintf(
		"https://api.hunter.io/v2/account?api_key=%s",
		url.QueryEscape(apiKey),
	)

	body, status, err := doGet(newClient(), apiURL)
	if err != nil {
		red.Printf("  [-] Hunter.io account fetch error: %v\n", err)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] Hunter.io account: HTTP %d\n", status)
		return nil
	}

	var parsed accountResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		red.Printf("  [-] Hunter.io account parse error: %v\n", err)
		return nil
	}

	d := parsed.Data
	return &AccountInfo{
		Email:      d.Email,
		FirstName:  d.FirstName,
		LastName:   d.LastName,
		PlanName:   d.PlanName,
		ResetDate:  d.ResetDate,
		SearchUsed: d.Requests.Searches.Used,
		SearchMax:  d.Requests.Searches.Available,
		VerifUsed:  d.Requests.Verifications.Used,
		VerifMax:   d.Requests.Verifications.Available,
	}
}

// PrintAccountInfo displays Hunter.io account details in a formatted box.
func PrintAccountInfo(info *AccountInfo) {
	cyan   := color.New(color.FgCyan, color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.FgHiBlack)

	dim.Println("  ┌─ Hunter.io ─────────────────────────────────────────────────")

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

	name := info.FirstName + " " + info.LastName
	row("Account", info.Email+"  ("+name+")")
	row("Plan", green.Sprint(info.PlanName))

	searchBar := limitBar(info.SearchUsed, info.SearchMax)
	row("Searches", fmt.Sprintf("%d / %d used  %s  (resets %s)",
		info.SearchUsed, info.SearchMax, searchBar, info.ResetDate))

	verifBar := limitBar(info.VerifUsed, info.VerifMax)
	row("Verifications", fmt.Sprintf("%d / %d used  %s",
		info.VerifUsed, info.VerifMax, verifBar))

	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search queries the Hunter.io domain-search endpoint.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain, apiKey string, seen *output.SeenSet) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying Hunter.io API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping Hunter.io — no API key provided")
		return nil
	}

	apiURL := fmt.Sprintf(
		"https://api.hunter.io/v2/domain-search?domain=%s&api_key=%s",
		url.QueryEscape(domain),
		url.QueryEscape(apiKey),
	)

	body, status, err := doGet(newClient(), apiURL)
	if err != nil {
		red.Printf("  [-] Hunter.io request error: %v\n", err)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] Hunter.io returned HTTP %d\n", status)
		return nil
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		red.Printf("  [-] Hunter.io parse error: %v\n", err)
		return nil
	}

	for _, e := range parsed.Errors {
		red.Printf("  [-] Hunter.io API error [%d]: %s\n", e.Code, e.Details)
	}

	var results []output.Result
	for _, e := range parsed.Data.Emails {
		if !seen.Add(e.Value) {
			continue // already found by a previous module — skip silently
		}
		r := output.Result{
			Email:  e.Value,
			Source: fmt.Sprintf("hunter.io (conf:%d%%)", e.Confidence),
		}
		results = append(results, r)
		output.PrintResult(e.Value, r.Source)
	}

	cyan.Printf("  [*] ")
	fmt.Printf("Hunter.io returned %d new emails", len(results))
	if parsed.Meta.Results > 0 {
		fmt.Printf("  (showing %d / %d total)", len(results), parsed.Meta.Results)
	}
	fmt.Println()

	return results
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func limitBar(used, max int) string {
	if max == 0 {
		return "[----------]"
	}
	filled := (used * 10) / max
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
	pct := (used * 100) / max
	return fmt.Sprintf("%s %d%%", bar, pct)
}
