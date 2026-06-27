// Package google provides a simple Google dorking module that scrapes
// DuckDuckGo HTML (as a Google alternative that doesn't block bots)
// to find email addresses associated with a target domain.
package google

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)

// dorks returns a list of search queries for the target domain
func dorks(domain string) []string {
	return []string{
		fmt.Sprintf(`"%s" email`, domain),
		fmt.Sprintf(`site:%s "email"`, domain),
		fmt.Sprintf(`"%s" contact`, domain),
		fmt.Sprintf(`"%s" "@%s"`, domain, domain),
		fmt.Sprintf(`inurl:%s mail`, domain),
	}
}

// Search queries DuckDuckGo for emails related to the domain.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain string, seen *output.SeenSet) []output.Result {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Printf("  [*] ")
	fmt.Println("Running search engine dork queries...")

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return nil
		},
	}

	var results []output.Result

	for _, dork := range dorks(domain) {
		searchURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(dork)

		req, _ := http.NewRequest("GET", searchURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")

		resp, err := client.Do(req)
		if err != nil {
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
		resp.Body.Close()

		for _, m := range emailRegex.FindAllString(string(body), -1) {
			m = strings.ToLower(m)
			if !strings.Contains(m, domain) {
				continue
			}
			if strings.HasSuffix(m, ".png") || strings.HasSuffix(m, ".jpg") {
				continue
			}
			if !seen.Add(m) {
				continue // duplicate — skip silently
			}
			r := output.Result{Email: m, Source: "dork-search"}
			results = append(results, r)
			output.PrintResult(m, "dork-search")
		}

		time.Sleep(1500 * time.Millisecond) // polite delay
	}

	return results
}
