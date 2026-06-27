// Package snovio integrates with the Snov.io Domain Search API (v2).
//
// Flow:
//  1. POST /v1/oauth/access_token (using client_id & client_secret) → receive access_token
//  2. POST /v2/domain-search/start → receive task_hash
//  3. GET  /v2/domain-search/domain-emails/result/{task_hash} → poll until status == "completed"
//  4. Follow pagination via the `next` cursor until exhausted.
package snovio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

const (
	tokenURL  = "https://api.snov.io/v1/oauth/access_token"
	startURL  = "https://api.snov.io/v2/domain-search/start"
	resultURL = "https://api.snov.io/v2/domain-search/domain-emails/result/%s"
	meURL     = "https://api.snov.io/v1/get-balance"

	pollInterval = 3 * time.Second
	maxPolls     = 20
)

// ── Account Info Types ────────────────────────────────────────────────────────

// AccountInfo holds Snov.io account credentials and usage limits.
type AccountInfo struct {
	UserID      string
	CreditsLeft int
}

type meResponse struct {
	Success bool `json:"success"`
	Balance int  `json:"balance"`
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
	Status string      `json:"status"`
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

// getAccessToken exchanges client_id (User ID) and client_secret (API Secret) for a token.
func getAccessToken(userID, apiSecret string) (string, error) {
	if userID == "" || apiSecret == "" {
		return "", fmt.Errorf("missing client credentials")
	}

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", userID)
	data.Set("client_secret", apiSecret)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	client := newClient()
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request HTTP %d: %s", resp.StatusCode, string(body))
	}

	var parsed struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("token parse error: %w", err)
	}

	if parsed.AccessToken == "" {
		return "", fmt.Errorf("received empty access token")
	}

	return parsed.AccessToken, nil
}

// ── Public API ────────────────────────────────────────────────────────────────

// GetAccountInfo fetches Snov.io account credentials and credit limits using the OAuth flow.
func GetAccountInfo(userID, apiSecret string) *AccountInfo {
	if userID == "" || apiSecret == "" {
		return nil
	}

	red := color.New(color.FgRed)
	token, err := getAccessToken(userID, apiSecret)
	if err != nil {
		red.Printf("  [-] Snov.io auth error: %v\n", err)
		return nil
	}

	req, err := http.NewRequest("GET", meURL, nil)
	if err != nil {
		red.Printf("  [-] Snov.io account request error: %v\n", err)
		return nil
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	client := newClient()
	resp, err := client.Do(req)
	if err != nil {
		red.Printf("  [-] Snov.io account fetch error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		red.Printf("  [-] Snov.io account: HTTP %d: %s\n", resp.StatusCode, string(body))
		return nil
	}

	var parsed meResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		red.Printf("  [-] Snov.io account parse error: %v\n", err)
		return nil
	}

	if !parsed.Success {
		red.Println("  [-] Snov.io account request unsuccessful")
		return nil
	}

	return &AccountInfo{
		UserID:      userID,
		CreditsLeft: parsed.Balance,
	}
}

// PrintAccountInfo displays Snov.io account details in a formatted box.
func PrintAccountInfo(info *AccountInfo) {
	cyan   := color.New(color.FgCyan, color.Bold)
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
		yellow.Println("API User ID or Secret not set — module will be skipped")
		dim.Println("  └─────────────────────────────────────────────────────────────")
		return
	}

	row("Account", info.UserID+"  (API Client ID)")
	row("Credits", fmt.Sprintf("%d remaining", info.CreditsLeft))

	dim.Println("  └─────────────────────────────────────────────────────────────")
}

// Search triggers a Snov.io domain-search job, polls for completion,
// and returns all discovered emails.
func Search(domain, userID, apiSecret string, seen *output.SeenSet) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying Snov.io API...")

	if userID == "" || apiSecret == "" {
		yellow.Println("  [!] Skipping Snov.io — credentials not fully provided")
		return nil
	}

	token, err := getAccessToken(userID, apiSecret)
	if err != nil {
		red.Printf("  [-] Snov.io auth error: %v\n", err)
		return nil
	}

	client := newClient()

	// ── Step 1: Start the search task ────────────────────────────────────────
	taskHash, err := startSearch(client, domain, token)
	if err != nil {
		red.Printf("  [-] Snov.io start error: %v\n", err)
		return nil
	}
	cyan.Printf("  [*] ")
	fmt.Printf("Snov.io task started (hash: %s)\n", taskHash)

	// ── Step 2: Poll for results ──────────────────────────────────────────────
	var allResults []output.Result
	nextCursor := ""
	totalCount := 0

	for poll := 0; poll < maxPolls; poll++ {
		time.Sleep(pollInterval)

		res, err := fetchResult(client, taskHash, token, nextCursor)
		if err != nil {
			red.Printf("  [-] Snov.io poll error: %v\n", err)
			break
		}

		if res.Meta.TotalCount > 0 {
			totalCount = res.Meta.TotalCount
		}

		for _, item := range res.Data {
			email := strings.ToLower(item.Email)
			if !seen.Add(email) {
				continue
			}
			r := output.Result{Email: email, Source: "snov.io"}
			allResults = append(allResults, r)
			output.PrintResult(email, "snov.io")
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
	fmt.Printf("Snov.io returned %d new emails", len(allResults))
	if totalCount > 0 && totalCount > len(allResults) {
		fmt.Printf("  (total available: %d)", totalCount)
	}
	fmt.Println()

	return allResults
}

// ── Internal helpers ──────────────────────────────────────────────────────────

func startSearch(client *http.Client, domain, token string) (string, error) {
	payload := map[string]string{
		"domain": domain,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", startURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusAccepted {
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

func fetchResult(client *http.Client, taskHash, token, nextCursor string) (*resultResponse, error) {
	var reqURL string
	if nextCursor != "" {
		// According to API docs pagination, we construct the next page URL
		reqURL = fmt.Sprintf(
			"https://api.snov.io/v2/domain-search/domain-emails/start?domain=&next=%s",
			nextCursor,
		)
	} else {
		reqURL = fmt.Sprintf(resultURL, taskHash)
	}

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
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
