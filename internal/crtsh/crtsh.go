// Package crtsh queries the crt.sh Certificate Transparency log
// to enumerate subdomains and extract email addresses.
package crtsh

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

type crtEntry struct {
	NameValue string `json:"name_value"`
}

// Lookup queries crt.sh for the given domain and returns subdomains + any
// emails found in the common-name fields.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Lookup(domain string, seen *output.SeenSet) ([]string, []output.Result) {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Printf("  [*] ")
	fmt.Printf("Querying crt.sh for subdomains of: %s\n", domain)

	apiURL := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", domain)

	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 EmailHunter/1.0")

	resp, err := client.Do(req)
	if err != nil {
		color.New(color.FgRed).Printf("  [-] crt.sh error: %v\n", err)
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		color.New(color.FgYellow).Printf("  [!] crt.sh: Server is overloaded or returned HTTP %d (skipping)\n", resp.StatusCode)
		return nil, nil
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := strings.TrimSpace(string(body))

	if strings.HasPrefix(bodyStr, "<") || !strings.HasPrefix(bodyStr, "[") {
		color.New(color.FgYellow).Println("  [!] crt.sh: Server is temporarily overloaded (returned HTML/error page)")
		return nil, nil
	}

	var entries []crtEntry
	if err := json.Unmarshal(body, &entries); err != nil {
		color.New(color.FgRed).Printf("  [-] Failed to parse crt.sh response: %v\n", err)
		return nil, nil
	}

	seenSubs := map[string]bool{}
	var subdomains []string
	var emails []output.Result

	for _, e := range entries {
		// Split multi-line name_value
		for _, name := range strings.Split(e.NameValue, "\n") {
			name = strings.TrimSpace(strings.TrimPrefix(name, "*."))

			// Collect subdomains
			if strings.HasSuffix(name, "."+domain) || name == domain {
				if !seenSubs[name] {
					seenSubs[name] = true
					subdomains = append(subdomains, name)
				}
			}

			// Collect emails sometimes embedded in SAN entries
			for _, m := range emailRegex.FindAllString(name, -1) {
				m = strings.ToLower(m)
				if !seen.Add(m) {
					continue // duplicate — skip silently
				}
				emails = append(emails, output.Result{Email: m, Source: "crt.sh"})
				output.PrintResult(m, "crt.sh")
			}
		}
	}

	cyan.Printf("  [*] ")
	fmt.Printf("Found %d subdomains via crt.sh\n", len(subdomains))

	return subdomains, emails
}
