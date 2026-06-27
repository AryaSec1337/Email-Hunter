// Package update handles checking for tool updates on GitHub and self-updating.
package update

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
)

const (
	versionURL = "https://raw.githubusercontent.com/AryaSec1337/Email-Hunter/main/version.txt"
	repoURL    = "github.com/AryaSec1337/Email-Hunter"
)

// CheckLatest queries the GitHub repository to check if a new version is available.
// It returns the latest version string and a boolean indicating if an update is available.
func CheckLatest(currentVersion string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", versionURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("User-Agent", "EmailHunter-Updater/1.0")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}

	latest := strings.TrimSpace(string(body))
	if latest == "" {
		return "", false
	}

	if isNewer(currentVersion, latest) {
		return latest, true
	}

	return latest, false
}

// UpdateSelf executes 'go install' to update the binary to the latest version.
func UpdateSelf() error {
	cyan  := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen, color.Bold)
	red   := color.New(color.FgRed, color.Bold)

	cyan.Printf("  [*] ")
	fmt.Println("Updating Email-Hunter to the latest version...")

	// Verify 'go' is installed and available
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("'go' executable not found in PATH. Please install Go (golang.org) to use automatic updates")
	}

	cyan.Printf("  [*] ")
	fmt.Printf("Running: go install -v %s@latest\n", repoURL)

	cmd := exec.Command("go", "install", "-v", repoURL+"@latest")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run go install: %w", err)
	}

	green.Printf("  [+] ")
	red.Println("Email-Hunter successfully updated to the latest version!")
	fmt.Println("      Re-run the tool to use the new version.")
	return nil
}

// isNewer parses 'vX.Y.Z' version strings and returns true if latest is newer than current.
func isNewer(current, latest string) bool {
	cParts := strings.Split(strings.TrimPrefix(current, "v"), ".")
	lParts := strings.Split(strings.TrimPrefix(latest, "v"), ".")

	for i := 0; i < len(cParts) && i < len(lParts); i++ {
		cVal, err1 := strconv.Atoi(cParts[i])
		lVal, err2 := strconv.Atoi(lParts[i])
		if err1 != nil || err2 != nil {
			// Fallback to string comparison if not numeric
			return latest > current
		}
		if lVal > cVal {
			return true
		}
		if cVal > lVal {
			return false
		}
	}

	// If prefixes match, the one with more version parts is newer (e.g. 1.2.1 > 1.2)
	return len(lParts) > len(cParts)
}
