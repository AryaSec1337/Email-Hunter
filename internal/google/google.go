// Package google provides a simple search engine dorking module that queries
// DuckDuckGo, Yahoo, and Google to find email addresses associated with a target domain.
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

// Search queries DuckDuckGo, Yahoo, and Google for emails related to the domain.
// seen is the global SeenSet — emails already found by other modules are skipped.
func Search(domain string, seen *output.SeenSet) []output.Result {
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Printf("  [*] ")
	fmt.Println("Running search engine dork queries (DuckDuckGo, Yahoo, Google)...")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	engines := []struct {
		name    string
		baseURL string
	}{
		{"DuckDuckGo", "https://html.duckduckgo.com/html/?q="},
		{"Yahoo", "https://search.yahoo.com/search?p="},
		{"Google", "https://www.google.com/search?q="},
	}

	var results []output.Result

	for _, engine := range engines {
		for _, dork := range dorks(domain) {
			searchURL := engine.baseURL + url.QueryEscape(dork)

			req, _ := http.NewRequest("GET", searchURL, nil)
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
			req.Header.Set("Accept-Language", "en-US,en;q=0.9")

			resp, err := client.Do(req)
			if err != nil {
				continue
			}

			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
			resp.Body.Close()

			bodyStr := string(body)

			// Detect bot verification block for Google
			if engine.name == "Google" && (strings.Contains(bodyStr, "google.com/recaptcha") || strings.Contains(bodyStr, "Jika Anda mengalami masalah saat mengakses Google")) {
				continue
			}

			for _, m := range emailRegex.FindAllString(bodyStr, -1) {
				m = strings.ToLower(m)
				if !strings.Contains(m, domain) {
					continue
				}
				if strings.HasSuffix(m, ".png") || strings.HasSuffix(m, ".jpg") || strings.HasSuffix(m, ".gif") || strings.HasSuffix(m, ".jpeg") {
					continue
				}
				if !seen.Add(m) {
					continue
				}
				r := output.Result{Email: m, Source: strings.ToLower(engine.name) + "-dork"}
				results = append(results, r)
				output.PrintResult(m, r.Source)
			}

			time.Sleep(1000 * time.Millisecond) // polite delay
		}
	}

	cyan.Printf("  [*] ")
	fmt.Printf("Search engine dorks returned %d new emails\n", len(results))

	return results
}
