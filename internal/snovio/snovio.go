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

	pollInterval = 3 * time.Second
	maxPolls     = 20 // up to ~60 seconds
)

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

// ── Public API ────────────────────────────────────────────────────────────────

// Search triggers a Snov.io domain-search job, polls for completion,
// and returns all discovered emails (paginated).
func Search(domain, apiKey string) []output.Result {
	cyan   := color.New(color.FgCyan, color.Bold)
	red    := color.New(color.FgRed)
	yellow := color.New(color.FgYellow)

	cyan.Printf("  [*] ")
	fmt.Println("Querying Snov.io API...")

	if apiKey == "" {
		yellow.Println("  [!] Skipping Snov.io — no API key provided (-snov-key)")
		return nil
	}

	client := &http.Client{Timeout: 20 * time.Second}

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

	for poll := 0; poll < maxPolls; poll++ {
		time.Sleep(pollInterval)

		res, err := fetchResult(client, taskHash, apiKey, nextCursor)
		if err != nil {
			red.Printf("  [-] Snov.io poll error: %v\n", err)
			break
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
			// Check if there are more pages
			if res.Links.Next != "" && res.Meta.Next != "" {
				nextCursor = res.Meta.Next
				// Reset poll counter for next page, but limit total pages
				if poll < maxPolls-1 {
					poll = 0
				}
				continue
			}
			// No more pages — done
			goto done

		case "failed":
			red.Println("  [-] Snov.io task failed")
			goto done

		default:
			// "queued" or "in_progress" — keep polling
			cyan.Printf("  [*] ")
			fmt.Printf("Snov.io status: %s (attempt %d/%d)\n", res.Status, poll+1, maxPolls)
		}
	}

done:
	cyan.Printf("  [*] ")
	fmt.Printf("Snov.io returned %d emails\n", len(allResults))

	return allResults
}

// ── Internal helpers ──────────────────────────────────────────────────────────

// startSearch POSTs to the start endpoint and returns the task_hash.
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

// fetchResult GETs the current results for a task.
// nextCursor is the pagination token from a previous response (empty = first page).
func fetchResult(client *http.Client, taskHash, apiKey, nextCursor string) (*resultResponse, error) {
	url := fmt.Sprintf(resultURL, taskHash)
	if nextCursor != "" {
		url = fmt.Sprintf(
			"https://api.snov.io/v2/domain-search/domain-emails/start?domain=&next=%s&api_key=%s",
			nextCursor, apiKey,
		)
	} else {
		url = url + "?api_key=" + apiKey
	}

	req, err := http.NewRequest("GET", url, nil)
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
