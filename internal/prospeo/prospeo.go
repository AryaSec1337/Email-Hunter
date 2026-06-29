// Package prospeo integrates with the Prospeo Domain Email Search API.
// Docs: https://prospeo.io/api-documentation
//
// Endpoint: POST https://api.prospeo.io/domain-email-search
// Auth:     X-KEY header
package prospeo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

const (
	searchURL  = "https://api.prospeo.io/domain-search"
	accountURL = "https://api.prospeo.io/account-information"
)

// ── Account Info ──────────────────────────────────────────────────────────────

// AccountInfo holds Prospeo account credentials and usage limits.
type AccountInfo struct {
	Email          string
	Plan           string
	CreditsUsed    int
	CreditsLimit   int
	DailyReqLeft   int
}

type accountResp struct {
	Response struct {
		Email string `json:"email"`
		Plan  struct {
			Name string `json:"name"`
		} `json:"plan"`
		Credits struct {
			Used  int `json:"used"`
			Limit int `json:"limit"`
		} `json:"credits"`
	} `json:"response"`
}

// ── Search Response ────────────────────────────────────────────────────────────

type emailItem struct {
	Email string `json:"email"`
	Type  string `json:"type"`
}

type searchResp struct {
	Response struct {
		EmailList  []emailItem `json:"email_list"`
		TotalCount int         `json:"total_count"`
	} `json:"response"`
	Error      interface{} `json:"error"`
	Message    string      `json:"message"`
	ErrorCode  string      `json:"error_code"`
	ErrorToast string      `json:"error_toast"`
}

// ── HTTP helper ────────────────────────────────────────────────────────────────

func newClient() *http.Client { return &http.Client{Timeout: 30 * time.Second} }

func doPost(client *http.Client, url, apiKey string, payload interface{}) ([]byte, http.Header, int, error) {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, nil, 0, err
	}
	req.Header.Set("X-KEY", apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, 0, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return b, resp.Header, resp.StatusCode, nil
}

func doGet(client *http.Client, url, apiKey string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("X-KEY", apiKey)
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

// GetAccountInfo fetches Prospeo account credentials and credit limits.
func GetAccountInfo(apiKey string) *AccountInfo {
	if apiKey == "" {
		return nil
	}
	red := color.New(color.FgRed)
	b, status, err := doGet(newClient(), accountURL, apiKey)
	if err != nil {
		red.Printf("  [-] Prospeo account fetch error: %v\n", err)
		return nil
	}
	if status != http.StatusOK {
		red.Printf("  [-] Prospeo account: HTTP %d\n", status)
		return nil
	}
	var parsed accountResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		red.Printf("  [-] Prospeo account parse error: %v\n", err)
		return nil
	}
	r := parsed.Response
	return &AccountInfo{
		Email:        r.Email,
		Plan:         r.Plan.Name,
		CreditsUsed:  r.Credits.Used,
		CreditsLimit: r.Credits.Limit,
	}
}

// PrintAccountInfo displays Prospeo account details in a formatted box.
func PrintAccountInfo(info *AccountInfo, inactive bool) {
	cyan   := color.New(color.FgCyan, color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.FgHiBlack)

	dim.Println("  ┌─ Prospeo ───────────────────────────────────────────────────")
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
	row("Account", info.Email)
	if info.Plan != "" {
		row("Plan", green.Sprint(info.Plan))
	}
	if info.CreditsLimit > 0 {
		bar := limitBar(info.CreditsUsed, info.CreditsLimit)
		remaining := info.CreditsLimit - info.CreditsUsed
		row("Credits", fmt.Sprintf("%d / %d used  %s  (%d remaining)",
			info.CreditsUsed, info.CreditsLimit, bar, remaining))
	}
	if info.DailyReqLeft > 0 {
		row("Daily Req Left", fmt.Sprintf("%d", info.DailyReqLeft))
	}
	if inactive {
		row("Status", yellow.Sprint("Inactive (disabled)"))
	} else {
		row("Status", green.Sprint("Active"))
	}
	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search queries the Prospeo domain-email-search endpoint.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain, apiKey string, seen *output.SeenSet) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying Prospeo API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping Prospeo — no API key provided")
		return nil
	}

	payload := map[string]interface{}{
		"url":   domain,
		"limit": 50,
	}

	b, headers, status, err := doPost(newClient(), searchURL, apiKey, payload)
	if err != nil {
		red.Printf("  [-] Prospeo request error: %v\n", err)
		return nil
	}
	if status == http.StatusTooManyRequests {
		red.Println("  [-] Prospeo: rate limit hit (429) — try again later")
		return nil
	}

	var parsed searchResp
	if err := json.Unmarshal(b, &parsed); err != nil {
		if status != http.StatusOK {
			red.Printf("  [-] Prospeo returned HTTP %d: %s\n", status, string(b))
		} else {
			red.Printf("  [-] Prospeo parse error: %v\n", err)
		}
		return nil
	}

	if parsed.ErrorCode == "DEPRECATED" {
		yellow.Println("  [!] Prospeo: The domain-search API endpoint has been deprecated/removed by Prospeo")
		return nil
	}

	if status != http.StatusOK {
		red.Printf("  [-] Prospeo returned HTTP %d: %s\n", status, string(b))
		return nil
	}

	// Extract daily request limit from headers
	dailyLeft := 0
	if v := headers.Get("x-daily-request-left"); v != "" {
		dailyLeft, _ = strconv.Atoi(v)
	}
	_ = dailyLeft
	if parsed.Error != nil {
		isError := false
		if errStr, ok := parsed.Error.(string); ok && errStr != "" {
			isError = true
		} else if errBool, ok := parsed.Error.(bool); ok && errBool {
			isError = true
		}
		if isError {
			errMsg := parsed.Message
			if errMsg == "" {
				errMsg = parsed.ErrorToast
			}
			red.Printf("  [-] Prospeo API error: %s\n", errMsg)
			return nil
		}
	}

	var results []output.Result
	for _, item := range parsed.Response.EmailList {
		email := strings.ToLower(item.Email)
		if email == "" {
			continue
		}
		if !seen.Add(email) {
			continue
		}
		r := output.Result{Email: email, Source: "prospeo.io"}
		results = append(results, r)
		output.PrintResult(email, r.Source)
	}

	cyan.Printf("  [*] ")
	fmt.Printf("Prospeo returned %d new emails", len(results))
	if parsed.Response.TotalCount > 0 {
		fmt.Printf("  (total available: %d)", parsed.Response.TotalCount)
	}
	if dailyLeft > 0 {
		fmt.Printf("  [daily req left: %d]", dailyLeft)
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
