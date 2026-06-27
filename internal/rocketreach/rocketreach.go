// Package rocketreach integrates with the RocketReach People Search API (v2).
// Docs: https://rocketreach.co/api
//
// Domain search strategy:
//  1. POST /v2/api/searchPeople → filter by current_employer domain
//  2. Extract emails from profiles[].emails[]
package rocketreach

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
	baseURL      = "https://api.rocketreach.co"
	searchURL    = baseURL + "/v2/person/search"
	accountURL   = baseURL + "/v2/api/account"
)

// ── Account Info ──────────────────────────────────────────────────────────────

// AccountInfo holds RocketReach account details and usage limits.
type AccountInfo struct {
	Name         string
	Email        string
	Plan         string
	LookupsMade  int
	LookupsLimit int
}

type accountResp struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Plan  struct {
		Name string `json:"name"`
	} `json:"plan"`
	LookupCredits struct {
		Used  int `json:"used"`
		Total int `json:"total"`
	} `json:"lookup_credits"`
}

// ── Search Response ────────────────────────────────────────────────────────────

type emailEntry struct {
	Email string `json:"email"`
	Type  string `json:"type"`
}

type profile struct {
	Name   string       `json:"name"`
	Emails []emailEntry `json:"emails"`
}

type searchResp struct {
	Profiles []profile `json:"profiles"`
	Pagination struct {
		Total int `json:"total"`
	} `json:"pagination"`
}

// ── HTTP helper ────────────────────────────────────────────────────────────────

func newClient() *http.Client { return &http.Client{Timeout: 20 * time.Second} }

func doRequest(client *http.Client, method, url, apiKey string, body []byte) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Api-Key", apiKey)
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

// GetAccountInfo fetches RocketReach account credentials and lookup limits.
func GetAccountInfo(apiKey string) *AccountInfo {
	if apiKey == "" {
		return nil
	}
	red := color.New(color.FgRed)
	b, status, err := doRequest(newClient(), "GET", accountURL, apiKey, nil)
	if err != nil {
		red.Printf("  [-] RocketReach account fetch error: %v\n", err)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] RocketReach account: HTTP %d\n", status)
		return nil
	}
	var parsed accountResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] RocketReach account parse error: %v\n", err)
		return nil
	}
	return &AccountInfo{
		Name:         parsed.Name,
		Email:        parsed.Email,
		Plan:         parsed.Plan.Name,
		LookupsMade:  parsed.LookupCredits.Used,
		LookupsLimit: parsed.LookupCredits.Total,
	}
}

// PrintAccountInfo displays RocketReach account details in a formatted box.
func PrintAccountInfo(info *AccountInfo) {
	cyan   := color.New(color.FgCyan, color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.FgHiBlack)

	dim.Println("  ┌─ RocketReach ───────────────────────────────────────────────")
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
	if info.Plan != "" {
		row("Plan", green.Sprint(info.Plan))
	}
	if info.LookupsLimit > 0 {
		bar := limitBar(info.LookupsMade, info.LookupsLimit)
		row("Lookups", fmt.Sprintf("%d / %d used  %s", info.LookupsMade, info.LookupsLimit, bar))
	}
	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search queries RocketReach for emails associated with the given domain.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain, apiKey string, seen *output.SeenSet) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying RocketReach API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping RocketReach — no API key provided")
		return nil
	}

	payload := map[string]interface{}{
		"query": map[string]interface{}{
			"current_employer": []string{domain},
		},
		"start":    1,
		"pageSize": 25,
	}
	body, _ := json.Marshal(payload)

	b, status, err := doRequest(newClient(), "POST", searchURL, apiKey, body)
	if err != nil {
		red.Printf("  [-] RocketReach request error: %v\n", err)
		return nil
	}
	if status != http.StatusOK && status != http.StatusCreated {
		red.Printf("  [-] RocketReach returned HTTP %d\n", status)
		return nil
	}

	var parsed searchResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] RocketReach parse error: %v\n", err)
		return nil
	}

	var results []output.Result
	for _, p := range parsed.Profiles {
		for _, e := range p.Emails {
			email := strings.ToLower(e.Email)
			if email == "" {
				continue
			}
			if !seen.Add(email) {
				continue
			}
			r := output.Result{Email: email, Source: "rocketreach.co"}
			results = append(results, r)
			output.PrintResult(email, r.Source)
		}
	}

	cyan.Printf("  [*] ")
	fmt.Printf("RocketReach returned %d new emails", len(results))
	if parsed.Pagination.Total > 0 {
		fmt.Printf("  (total available: %d)", parsed.Pagination.Total)
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
