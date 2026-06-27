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

// Config holds all loaded configuration values.
type Config struct {
	HunterAPIKey string
	SnovAPIKey   string
}

// ConfigPath returns the canonical path to the config file.
// On all platforms this resolves to: <home>/.config/.config-emailhunter
func ConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", configFileName), nil
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
		// Config file is optional — not an error
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

		// Skip comments and blank lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			// Warn but continue
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
		default:
			// Unknown key — silently skip to allow future additions
		}
	}

	if err := scanner.Err(); err != nil {
		return cfg, fmt.Errorf("error reading config file: %w", err)
	}

	return cfg, nil
}

// PrintStatus prints which keys were successfully loaded from the config file.
func PrintStatus(cfg *Config) {
	path, _ := ConfigPath()
	cyan  := color.New(color.FgCyan, color.Bold)
	green := color.New(color.FgGreen)
	dim   := color.New(color.FgHiBlack)

	cyan.Printf("  [*] ")
	fmt.Printf("Config file: %s\n", path)

	printKey := func(name, val string) {
		cyan.Printf("  [*] ")
		fmt.Printf("%-18s ", name+":")
		if val != "" {
			masked := maskKey(val)
			green.Printf("loaded (%s)\n", masked)
		} else {
			dim.Println("not set")
		}
	}

	printKey("HUNTER_API_KEY", cfg.HunterAPIKey)
	printKey("SNOV_API_KEY",   cfg.SnovAPIKey)
}

// maskKey shows only the first 4 and last 4 characters of a key.
func maskKey(key string) string {
	if len(key) <= 8 {
		return strings.Repeat("*", len(key))
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}
