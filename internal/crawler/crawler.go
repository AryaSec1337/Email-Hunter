package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"

	"github.com/AryaSec1337/Email-Hunter/internal/output"
)

var (
	emailRegex = regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`)
	linkRegex  = regexp.MustCompile(`href="(https?://[^"]+)"`)
)

// Crawler holds crawler state
type Crawler struct {
	Domain     string
	MaxDepth   int
	MaxPages   int
	Timeout    time.Duration
	client     *http.Client
	visited    map[string]bool
	mu         sync.Mutex
	Results    []output.Result
}

// New creates a new crawler for the given domain
func New(domain string, maxDepth, maxPages int) *Crawler {
	return &Crawler{
		Domain:   domain,
		MaxDepth: maxDepth,
		MaxPages: maxPages,
		Timeout:  15 * time.Second,
		visited:  make(map[string]bool),
		client: &http.Client{
			Timeout: 15 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// Run starts crawling and returns discovered emails.
// seen is the global SeenSet — emails already found by other modules are skipped.
func (c *Crawler) Run(seen *output.SeenSet) []output.Result {
	startURL := "https://" + c.Domain
	cyan := color.New(color.FgCyan, color.Bold)
	cyan.Printf("  [*] ")
	fmt.Printf("Starting web crawl on: %s\n", startURL)

	var wg sync.WaitGroup
	pageCh  := make(chan string, 100)
	emailCh := make(chan output.Result, 500)

	// Collector goroutine: dedup via global SeenSet before printing
	go func() {
		for r := range emailCh {
			if !seen.Add(r.Email) {
				continue // duplicate — skip silently
			}
			c.mu.Lock()
			c.Results = append(c.Results, r)
			c.mu.Unlock()
			output.PrintResult(r.Email, r.Source)
		}
	}()

	pageCh <- startURL
	c.mu.Lock()
	c.visited[startURL] = true
	c.mu.Unlock()

	workers := 5
	sem := make(chan struct{}, workers)

	for i := 0; i < c.MaxPages; i++ {
		pageURL, ok := <-pageCh
		if !ok {
			break
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(u string) {
			defer wg.Done()
			defer func() { <-sem }()

			emails, links := c.fetchPage(u)
			for _, e := range emails {
				emailCh <- output.Result{Email: e, Source: "web-crawl"}
			}

			c.mu.Lock()
			pagesVisited := len(c.visited)
			c.mu.Unlock()

			for _, link := range links {
				c.mu.Lock()
				if !c.visited[link] && pagesVisited < c.MaxPages {
					c.visited[link] = true
					select {
					case pageCh <- link:
					default:
					}
				}
				c.mu.Unlock()
			}
		}(pageURL)
	}

	wg.Wait()
	close(emailCh)
	time.Sleep(100 * time.Millisecond) // let collector finish

	return c.Results
}

// fetchPage fetches a URL and extracts emails and links
func (c *Crawler) fetchPage(pageURL string) ([]string, []string) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; EmailHunter/1.0)")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB limit
	if err != nil {
		return nil, nil
	}

	content := string(body)
	emails := extractEmails(content, c.Domain)
	links := extractLinks(content, pageURL, c.Domain)

	return emails, links
}

// extractEmails finds emails in content that match the target domain
func extractEmails(content, domain string) []string {
	matches := emailRegex.FindAllString(content, -1)
	seen := map[string]bool{}
	var result []string

	for _, m := range matches {
		m = strings.ToLower(m)
		// Filter noise
		if strings.HasSuffix(m, ".png") || strings.HasSuffix(m, ".jpg") ||
			strings.HasSuffix(m, ".css") || strings.HasSuffix(m, ".js") {
			continue
		}
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

// extractLinks finds internal links on a page
func extractLinks(content, baseURL, domain string) []string {
	matches := linkRegex.FindAllStringSubmatch(content, -1)
	var links []string

	base, _ := url.Parse(baseURL)

	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		link := m[1]
		u, err := url.Parse(link)
		if err != nil {
			continue
		}
		// Resolve relative URLs
		resolved := base.ResolveReference(u)
		if strings.Contains(resolved.Host, domain) {
			links = append(links, resolved.String())
		}
	}
	return links
}
