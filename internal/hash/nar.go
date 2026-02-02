// Package hash provides functionality for computing Nix-compatible hashes.
package hash

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// ComputeNARHash computes the Nix NAR hash of a directory.
// It first tries to use the nix command if available, otherwise falls back
// to a pure Go implementation.
func ComputeNARHash(path string) (string, error) {
	// Try using nix hash path first (most accurate)
	if hash, err := computeWithNix(path); err == nil {
		return hash, nil
	}

	// Fall back to pure Go NAR implementation
	return computeNARHashGo(path)
}

// computeWithNix uses the nix command to compute the hash.
func computeWithNix(path string) (string, error) {
	cmd := exec.Command("nix", "hash", "path", "--sri", path)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// computeNARHashGo computes a NAR hash using pure Go.
// NAR (Nix Archive) format is a deterministic archive format.
func computeNARHashGo(path string) (string, error) {
	h := sha256.New()
	if err := writeNAR(h, path); err != nil {
		return "", fmt.Errorf("computing NAR: %w", err)
	}

	hash := h.Sum(nil)
	// Convert to SRI format: sha256-<base64>
	return "sha256-" + base64.StdEncoding.EncodeToString(hash), nil
}

// writeNAR writes the NAR representation of path to w.
// NAR format specification: https://nixos.org/manual/nix/stable/protocols/nix-archive-format.html
func writeNAR(w io.Writer, path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}

	// Write NAR header
	if _, err := w.Write([]byte("nix-archive-1")); err != nil {
		return err
	}
	if err := writeString(w, ""); err != nil { // padding
		return err
	}

	return writeNAREntry(w, path, info)
}

func writeNAREntry(w io.Writer, path string, info os.FileInfo) error {
	if err := writeString(w, "("); err != nil {
		return err
	}

	if err := writeString(w, "type"); err != nil {
		return err
	}

	switch {
	case info.Mode().IsRegular():
		if err := writeString(w, "regular"); err != nil {
			return err
		}

		// Check if executable
		if info.Mode()&0111 != 0 {
			if err := writeString(w, "executable"); err != nil {
				return err
			}
			if err := writeString(w, ""); err != nil {
				return err
			}
		}

		// Write contents
		if err := writeString(w, "contents"); err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := writeBytes(w, data); err != nil {
			return err
		}

	case info.IsDir():
		if err := writeString(w, "directory"); err != nil {
			return err
		}

		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		// NAR requires sorted entries
		sort.Slice(entries, func(i, j int) bool {
			return entries[i].Name() < entries[j].Name()
		})

		for _, entry := range entries {
			if err := writeString(w, "entry"); err != nil {
				return err
			}
			if err := writeString(w, "("); err != nil {
				return err
			}
			if err := writeString(w, "name"); err != nil {
				return err
			}
			if err := writeString(w, entry.Name()); err != nil {
				return err
			}
			if err := writeString(w, "node"); err != nil {
				return err
			}

			entryPath := filepath.Join(path, entry.Name())
			entryInfo, err := os.Lstat(entryPath)
			if err != nil {
				return err
			}
			if err := writeNAREntry(w, entryPath, entryInfo); err != nil {
				return err
			}

			if err := writeString(w, ")"); err != nil {
				return err
			}
		}

	case info.Mode()&os.ModeSymlink != 0:
		if err := writeString(w, "symlink"); err != nil {
			return err
		}
		if err := writeString(w, "target"); err != nil {
			return err
		}
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		if err := writeString(w, target); err != nil {
			return err
		}

	default:
		return fmt.Errorf("unsupported file type: %s", info.Mode())
	}

	return writeString(w, ")")
}

// writeString writes a NAR string (length-prefixed, padded to 8 bytes).
func writeString(w io.Writer, s string) error {
	return writeBytes(w, []byte(s))
}

// writeBytes writes NAR bytes (length-prefixed, padded to 8 bytes).
func writeBytes(w io.Writer, data []byte) error {
	// Write length as 64-bit little-endian
	length := uint64(len(data))
	lengthBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		lengthBytes[i] = byte(length >> (i * 8))
	}
	if _, err := w.Write(lengthBytes); err != nil {
		return err
	}

	// Write data
	if _, err := w.Write(data); err != nil {
		return err
	}

	// Pad to 8-byte boundary
	padding := (8 - (len(data) % 8)) % 8
	if padding > 0 {
		if _, err := w.Write(bytes.Repeat([]byte{0}, padding)); err != nil {
			return err
		}
	}

	return nil
}

// ToSRI converts a raw SHA256 hash to SRI format.
func ToSRI(hash []byte) string {
	return "sha256-" + base64.StdEncoding.EncodeToString(hash)
}
