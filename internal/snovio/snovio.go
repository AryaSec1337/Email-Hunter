// Package snovio integrates with the Snov.io Domain Search API (v2).
//
// Flow:
//  1. POST /v2/domain-search/start  → receive task_hash
//  2. GET  /v2/domain-search/domain-emails/result/{task_hash}
//     → poll until status == "completed"
//  3. Follow pagination via the `next` cursor until exhausted.
package snovio

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
	startURL  = "https://api.snov.io/v2/domain-search/start"
	resultURL = "https://api.snov.io/v2/domain-search/domain-emails/result/%s"
	meURL     = "https://api.snov.io/v2/me"

	pollInterval = 3 * time.Second
	maxPolls     = 20 // up to ~60 seconds
)

// ── Account Info Types ────────────────────────────────────────────────────────

// AccountInfo holds Snov.io account credentials and usage limits.
type AccountInfo struct {
	Email          string
	Name           string
	Plan           string
	CreditsLeft    int
	CreditsTotal   int
	CreditsUsed    int
}

type meResponse struct {
	Data struct {
		Email        string `json:"email"`
		Name         string `json:"name"`
		Plan         string `json:"plan"`
		CreditsLeft  int    `json:"credits_left"`
		CreditsTotal int    `json:"credits_total"`
		CreditsUsed  int    `json:"credits_used"`
	} `json:"data"`
	// Some Snov.io endpoints return top-level fields
	Email        string `json:"email"`
	Name         string `json:"name"`
	Plan         string `json:"plan"`
	CreditsLeft  int    `json:"credits_left"`
	CreditsTotal int    `json:"credits_total"`
}

// ── Response types ────────────────────────────────────────────────────────────

type startResponse struct {
	Data interface{} `json:"data"`
	Meta struct {
		Domain   string `json:"domain"`
		TaskHash string `json:"task_hash"`
	} `json:"meta"`
	Links struct {
		Result string `json:"result"`
	} `json:"links"`
}

type emailItem struct {
	Email string `json:"email"`
}

type resultResponse struct {
	Data   []emailItem `json:"data"`
	Status string      `json:"status"` // "queued" | "in_progress" | "completed" | "failed"
	Meta   struct {
		Domain     string `json:"domain"`
		TaskHash   string `json:"task_hash"`
		Next       string `json:"next"`
		TotalCount int    `json:"total_count"`
	} `json:"meta"`
	Links struct {
		Next string `json:"next"`
	} `json:"links"`
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

func newClient() *http.Client {
	return &http.Client{Timeout: 20 * time.Second}
}

// ── Public API ────────────────────────────────────────────────────────────────

// GetAccountInfo fetches Snov.io account credentials and credit limits.
// Returns nil (with a printed warning) when the key is empty or the call fails.
func GetAccountInfo(apiKey string) *AccountInfo {
	if apiKey == "" {
		return nil
	}

	red := color.New(color.FgRed)
	client := newClient()

	// Snov.io /v2/me accepts api_key as query param
	reqURL := meURL + "?api_key=" + apiKey
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		red.Printf("  [-] Snov.io account fetch error: %v\n", err)
		return nil
	}
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		red.Printf("  [-] Snov.io account fetch error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		red.Printf("  [-] Snov.io account: HTTP %d\n", resp.StatusCode)
		return nil
	}

	var parsed meResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		red.Printf("  [-] Snov.io account parse error: %v\n", err)
		return nil
	}

	// Handle both nested (.data) and flat response formats
	info := &AccountInfo{}
	if parsed.Data.Email != "" {
		info.Email        = parsed.Data.Email
		info.Name         = parsed.Data.Name
		info.Plan         = parsed.Data.Plan
		info.CreditsLeft  = parsed.Data.CreditsLeft
		info.CreditsTotal = parsed.Data.CreditsTotal
		info.CreditsUsed  = parsed.Data.CreditsUsed
	} else {
		info.Email        = parsed.Email
		info.Name         = parsed.Name
		info.Plan         = parsed.Plan
		info.CreditsLeft  = parsed.CreditsLeft
		info.CreditsTotal = parsed.CreditsTotal
	}

	if info.Email == "" {
		// Endpoint might not exist or returned unexpected format
		red.Println("  [-] Snov.io account: no user data in response")
		return nil
	}

	return info
}

// PrintAccountInfo displays Snov.io account details in a formatted box.
func PrintAccountInfo(info *AccountInfo) {
	cyan   := color.New(color.FgCyan, color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.FgHiBlack)

	dim.Println("  ┌─ Snov.io ───────────────────────────────────────────────────")

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

	if info.CreditsTotal > 0 {
		bar := limitBar(info.CreditsUsed, info.CreditsTotal)
		row("Credits", fmt.Sprintf("%d / %d used  %s  (%d remaining)",
			info.CreditsUsed, info.CreditsTotal, bar, info.CreditsLeft))
	} else if info.CreditsLeft > 0 {
		row("Credits", fmt.Sprintf("%d remaining", info.CreditsLeft))
	}

	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search triggers a Snov.io domain-search job, polls for completion,
// and returns all discovered emails (paginated).
func Search(domain, apiKey string) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying Snov.io API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping Snov.io — no API key provided")
		return nil
	}

	client := newClient()

	// ── Step 1: Start the search task ────────────────────────────────────────
	taskHash, err := startSearch(client, domain, apiKey)
	if err != nil {
		red.Printf("  [-] Snov.io start error: %v\n", err)
		return nil
	}
	cyan.Printf("  [*] ")
	fmt.Printf("Snov.io task started (hash: %s)\n", taskHash)

	// ── Step 2: Poll for results ──────────────────────────────────────────────
	var allResults []output.Result
	seen := map[string]bool{}
	nextCursor := ""
	totalCount := 0

	for poll := 0; poll < maxPolls; poll++ {
		time.Sleep(pollInterval)

		res, err := fetchResult(client, taskHash, apiKey, nextCursor)
		if err != nil {
			red.Printf("  [-] Snov.io poll error: %v\n", err)
			break
		}

		if res.Meta.TotalCount > 0 {
			totalCount = res.Meta.TotalCount
		}

		// Collect emails from this page
		for _, item := range res.Data {
			email := strings.ToLower(item.Email)
			if !seen[email] {
				seen[email] = true
				r := output.Result{Email: email, Source: "snov.io"}
				allResults = append(allResults, r)
				output.PrintResult(email, "snov.io")
			}
		}

		switch res.Status {
		case "completed":
			if res.Links.Next != "" && res.Meta.Next != "" {
				nextCursor = res.Meta.Next
				if poll < maxPolls-1 {
					poll = 0
				}
				continue
			}
			goto done

		case "failed":
			red.Println("  [-] Snov.io task failed")
			goto done

		default:
			cyan.Printf("  [*] ")
			fmt.Printf("Snov.io status: %s (attempt %d/%d)\n", res.Status, poll+1, maxPolls)
		}
	}

done:
	cyan.Printf("  [*] ")
	fmt.Printf("Snov.io returned %d emails", len(allResults))
	if totalCount > 0 && totalCount > len(allResults) {
		fmt.Printf("  (total available: %d)", totalCount)
	}
	fmt.Println()

	return allResults
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func startSearch(client *http.Client, domain, apiKey string) (string, error) {
	payload := map[string]string{
		"domain":  domain,
		"api_key": apiKey,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", startURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed startResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse error: %w", err)
	}

	if parsed.Meta.TaskHash == "" {
		return "", fmt.Errorf("empty task_hash in response: %s", string(respBody))
	}

	return parsed.Meta.TaskHash, nil
}

func fetchResult(client *http.Client, taskHash, apiKey, nextCursor string) (*resultResponse, error) {
	var reqURL string
	if nextCursor != "" {
		reqURL = fmt.Sprintf(
			"https://api.snov.io/v2/domain-search/domain-emails/start?domain=&next=%s&api_key=%s",
			nextCursor, apiKey,
		)
	} else {
		reqURL = fmt.Sprintf(resultURL, taskHash) + "?api_key=" + apiKey
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed resultResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse error: %w (body: %s)", err, string(body))
	}

	return &parsed, nil
}

// limitBar returns a small ASCII progress bar showing used/total ratio.
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
