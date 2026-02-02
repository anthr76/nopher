// Package fetch provides functionality for fetching Go modules.
package fetch

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NetrcEntry represents a single machine entry in a .netrc file.
type NetrcEntry struct {
	Machine  string
	Login    string
	Password string
}

// Netrc represents a parsed .netrc file.
type Netrc struct {
	Entries []NetrcEntry
}

// ParseNetrc reads and parses the user's .netrc file.
func ParseNetrc() (*Netrc, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	return ParseNetrcFile(filepath.Join(home, ".netrc"))
}

// ParseNetrcFile parses a .netrc file from the given path.
func ParseNetrcFile(path string) (*Netrc, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No .netrc file is not an error
			return &Netrc{}, nil
		}
		return nil, fmt.Errorf("opening .netrc: %w", err)
	}
	defer f.Close()

	var netrc Netrc
	var current *NetrcEntry

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tokens := tokenize(line)
		for i := 0; i < len(tokens); i++ {
			switch tokens[i] {
			case "machine":
				if current != nil {
					netrc.Entries = append(netrc.Entries, *current)
				}
				current = &NetrcEntry{}
				if i+1 < len(tokens) {
					current.Machine = tokens[i+1]
					i++
				}
			case "default":
				if current != nil {
					netrc.Entries = append(netrc.Entries, *current)
				}
				current = &NetrcEntry{Machine: ""}
			case "login":
				if current != nil && i+1 < len(tokens) {
					current.Login = tokens[i+1]
					i++
				}
			case "password":
				if current != nil && i+1 < len(tokens) {
					current.Password = tokens[i+1]
					i++
				}
			case "account":
				// Skip account field
				if i+1 < len(tokens) {
					i++
				}
			case "macdef":
				// Skip macro definitions
				return &netrc, nil
			}
		}
	}

	if current != nil {
		netrc.Entries = append(netrc.Entries, *current)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning .netrc: %w", err)
	}

	return &netrc, nil
}

// FindEntry finds the netrc entry for the given machine.
func (n *Netrc) FindEntry(machine string) *NetrcEntry {
	// First, look for exact match
	for _, e := range n.Entries {
		if e.Machine == machine {
			return &e
		}
	}

	// Fall back to default entry
	for _, e := range n.Entries {
		if e.Machine == "" {
			return &e
		}
	}

	return nil
}

// tokenize splits a netrc line into tokens, handling quotes.
func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for _, r := range line {
		switch {
		case r == '"':
			inQuote = !inQuote
		case r == ' ' || r == '\t':
			if inQuote {
				current.WriteRune(r)
			} else if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteRune(r)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}
