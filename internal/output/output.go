package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

// Result holds a discovered email and its source
type Result struct {
	Email  string `json:"email"`
	Source string `json:"source"`
}

// Results is a collection of email results
type Results struct {
	Domain    string   `json:"domain"`
	Timestamp string   `json:"timestamp"`
	Count     int      `json:"count"`
	Emails    []Result `json:"emails"`
}

// SeenSet is a concurrency-safe global deduplication tracker.
// Pass a single instance to all modules so no email is printed or
// collected twice — even across different sources running concurrently.
type SeenSet struct {
	mu   sync.Mutex
	seen map[string]bool
}

// NewSeenSet creates an empty SeenSet ready for use.
func NewSeenSet() *SeenSet {
	return &SeenSet{seen: make(map[string]bool)}
}

// Add returns true and records the email if it has NOT been seen before.
// Returns false (and does nothing) if the email is already known.
// Comparison is case-insensitive; the normalised key is stored.
func (s *SeenSet) Add(email string) bool {
	key := strings.ToLower(strings.TrimSpace(email))
	if key == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.seen[key] {
		return false
	}
	s.seen[key] = true
	return true
}

// Len returns the number of unique emails recorded so far.
func (s *SeenSet) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.seen)
}

// PrintResult is a no-op during scanning. Emails are printed in PrintSummary.
func PrintResult(email, source string) {}

// PrintSummary prints a summary of all found emails
func PrintSummary(emails []Result, domain string) {
	fmt.Println()
	color.New(color.FgHiBlack).Println("  ─────────────────────────────────────────────────────────────────────")
	fmt.Println()

	yellow := color.New(color.FgYellow, color.Bold)
	cyan := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	dim := color.New(color.FgCyan)

	cyan.Printf("  [*] ")
	fmt.Printf("Scan complete for domain: ")
	yellow.Println(domain)

	cyan.Printf("  [*] ")
	fmt.Printf("Total unique emails found: ")
	yellow.Printf("%d\n\n", len(emails))

	if len(emails) > 0 {
		for _, e := range emails {
			green.Printf("  [+] ")
			fmt.Printf("%-40s", e.Email)
			dim.Printf("  [%s]\n", e.Source)
		}
		fmt.Println()
	}
}

// SaveTXT saves results to a text file
func SaveTXT(emails []Result, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	for _, r := range emails {
		fmt.Fprintf(f, "%s [source: %s]\n", r.Email, r.Source)
	}
	return nil
}

// SaveJSON saves results to a JSON file
func SaveJSON(emails []Result, domain, filename string) error {
	results := Results{
		Domain:    domain,
		Timestamp: time.Now().Format(time.RFC3339),
		Count:     len(emails),
		Emails:    emails,
	}

	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// SaveCSV saves results to a CSV file
func SaveCSV(emails []Result, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	_ = w.Write([]string{"email", "source"})
	for _, r := range emails {
		_ = w.Write([]string{r.Email, r.Source})
	}
	return nil
}

// Deduplicate removes duplicate emails (case-insensitive).
// This is kept as a safety net for any results that bypass SeenSet,
// but with proper SeenSet usage the list should already be unique.
func Deduplicate(results []Result) []Result {
	seen := make(map[string]bool)
	unique := []Result{}

	for _, r := range results {
		key := strings.ToLower(r.Email)
		if !seen[key] {
			seen[key] = true
			unique = append(unique, r)
		}
	}
	return unique
}
