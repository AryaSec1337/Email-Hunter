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

// ── API Response Types ────────────────────────────────────────────────────────

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
	Errors []struct {
		ID      string `json:"id"`
		Code    int    `json:"code"`
		Details string `json:"details"`
	} `json:"errors"`
}

// ── Public API ────────────────────────────────────────────────────────────────

// Search queries the Hunter.io domain-search endpoint and returns
// all email addresses associated with the target domain.
func Search(domain, apiKey string) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying Hunter.io API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping Hunter.io — no API key provided (-hunter-key)")
		return nil
	}

	apiURL := fmt.Sprintf(
		"https://api.hunter.io/v2/domain-search?domain=%s&api_key=%s",
		url.QueryEscape(domain),
		url.QueryEscape(apiKey),
	)

	client := &http.Client{Timeout: 20 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		red.Printf("  [-] Hunter.io request build error: %v\n", err)
		return nil
	}
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		red.Printf("  [-] Hunter.io request error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		red.Printf("  [-] Hunter.io returned HTTP %d\n", resp.StatusCode)
		return nil
	}

	var parsed apiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		red.Printf("  [-] Hunter.io parse error: %v\n", err)
		return nil
	}

	// Report any API-level errors
	for _, e := range parsed.Errors {
		red.Printf("  [-] Hunter.io API error [%d]: %s\n", e.Code, e.Details)
	}

	var results []output.Result
	for _, e := range parsed.Data.Emails {
		r := output.Result{
			Email:  e.Value,
			Source: fmt.Sprintf("hunter.io (conf:%d%%)", e.Confidence),
		}
		results = append(results, r)
		output.PrintResult(e.Value, r.Source)
	}

	cyan.Printf("  [*] ")
	fmt.Printf("Hunter.io returned %d emails\n", len(results))

	return results
}
