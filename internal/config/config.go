// Package config handles loading Email-Hunter configuration from a flat key=value
// file located at ~/.config/.config-emailhunter (cross-platform).
//
// File format:
//
//	HUNTER_API_KEY=your_hunter_key_here
//	SNOV_API_KEY=your_snov_key_here
//
// Lines starting with '#' are treated as comments and ignored.
// Blank lines are ignored.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
)

const configFileName = ".config-emailhunter"

// defaultTemplate is written to a new config file on first run.
const defaultTemplate = `# ============================================================
#  Email-Hunter Configuration File
#  Auto-generated on first run.
#
#  Fill in your API keys below, then re-run the tool.
#
#  Get your keys at:
#    Hunter.io  -> https://hunter.io/api-keys
#    Snov.io    -> https://app.snov.io/account?settings=api
# ============================================================

# Hunter.io API Key
HUNTER_API_KEY=

# Snov.io API Key
SNOV_API_KEY=
`

// Config holds all loaded configuration values.
type Config struct {
	HunterAPIKey string
	SnovAPIKey   string
}

// SetupResult describes what happened during Setup().
type SetupResult int

const (
	SetupAlreadyExists SetupResult = iota // Config file already present
	SetupCreated                          // Config file was newly created
	SetupFailed                           // Could not create file/dir
)

// ConfigDir returns the directory that holds the config file.
func ConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

// ConfigPath returns the canonical path to the config file.
// On all platforms this resolves to: <home>/.config/.config-emailhunter
func ConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}

// Setup ensures the config directory and file exist.
// On first run it creates them automatically and returns SetupCreated.
// If the file already exists it returns SetupAlreadyExists.
func Setup() (SetupResult, string, error) {
	path, err := ConfigPath()
	if err != nil {
		return SetupFailed, "", err
	}

	// File already exists — nothing to do
	if _, err := os.Stat(path); err == nil {
		return SetupAlreadyExists, path, nil
	}

	// Create ~/.config directory (no-op if already present)
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return SetupFailed, path, fmt.Errorf("cannot create config directory %s: %w", dir, err)
	}

	// Write default template
	if err := os.WriteFile(path, []byte(defaultTemplate), 0600); err != nil {
		return SetupFailed, path, fmt.Errorf("cannot write config file: %w", err)
	}

	return SetupCreated, path, nil
}

// Load reads the config file and returns a populated Config.
// It is NOT an error if the file does not exist — an empty Config is returned.
func Load() (*Config, error) {
	path, err := ConfigPath()
	if err != nil {
		return &Config{}, err
	}

	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return &Config{}, fmt.Errorf("failed to open config file: %w", err)
	}
	defer f.Close()

	cfg := &Config{}
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			color.New(color.FgYellow).Printf(
				"  [!] config: ignoring malformed line %d: %q\n", lineNum, line,
			)
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "HUNTER_API_KEY":
			cfg.HunterAPIKey = val
		case "SNOV_API_KEY":
			cfg.SnovAPIKey = val
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("error reading config file: %w", err)
	}

	return cfg, nil
}

// PrintStatus prints which keys were loaded and warns about missing ones.
// missingModules lists modules that are enabled but have no key.
func PrintStatus(cfg *Config, noHunter, noSnov bool) {
	path, _ := ConfigPath()
	cyan   := color.New(color.FgCyan, color.Bold)
	green  := color.New(color.FgGreen)
	yellow := color.New(color.FgYellow, color.Bold)
	dim    := color.New(color.FgHiBlack)

	cyan.Printf("  [*] ")
	fmt.Printf("Config : %s\n", path)

	printKey := func(label, val string, disabled bool) {
		cyan.Printf("  [*] ")
		fmt.Printf("  %-18s ", label+":")
		switch {
		case disabled:
			dim.Println("disabled (--no-* flag)")
		case val != "":
			green.Printf("✔  loaded (%s)\n", maskKey(val))
		default:
			yellow.Println("✘  not set")
		}
	}

	printKey("HUNTER_API_KEY", cfg.HunterAPIKey, noHunter)
	printKey("SNOV_API_KEY",   cfg.SnovAPIKey,   noSnov)
}

// WarnMissingKeys prints actionable warnings for any enabled module
// whose API key is still empty after merging CLI flags + config file.
// Returns true if at least one API key is missing for an enabled module.
func WarnMissingKeys(cfg *Config, noHunter, noSnov bool) bool {
	path, _ := ConfigPath()
	yellow := color.New(color.FgYellow, color.Bold)
	dim    := color.New(color.FgHiBlack)
	warned := false

	warn := func(name, envKey string) {
		warned = true
		yellow.Printf("\n  [!] %s API key is not set.\n", name)
		dim.Printf("      Edit your config file and fill in %s:\n", envKey)
		dim.Printf("      %s\n", path)
	}

	if !noHunter && cfg.HunterAPIKey == "" {
		warn("Hunter.io", "HUNTER_API_KEY")
	}
	if !noSnov && cfg.SnovAPIKey == "" {
		warn("Snov.io", "SNOV_API_KEY")
	}

	if warned {
		fmt.Println()
		yellow.Println("  [!] API modules with missing keys will be skipped automatically.")
		dim.Println("      Use --no-hunter / --no-snov to suppress these warnings.")
		fmt.Println()
	}

	return warned
}

// maskKey shows only the first 4 and last 4 characters of a key.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
