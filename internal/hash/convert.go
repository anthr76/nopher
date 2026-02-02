package hash

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// ConvertGoH1ToSRI attempts to convert a Go h1: hash to SRI format.
// Note: Go's h1: hash is SHA256 of the module zip, NOT the NAR hash.
// This function is mainly for reference/validation, not for Nix hashes.
func ConvertGoH1ToSRI(h1 string) (string, error) {
	if !strings.HasPrefix(h1, "h1:") {
		return "", fmt.Errorf("invalid h1 hash format: %s", h1)
	}

	// h1: is base64-encoded SHA256
	b64 := strings.TrimPrefix(h1, "h1:")
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", fmt.Errorf("decoding h1 hash: %w", err)
	}

	if len(decoded) != 32 {
		return "", fmt.Errorf("invalid h1 hash length: %d", len(decoded))
	}

	// Return as SRI format (though this won't match NAR hash)
	return "sha256-" + b64, nil
}

// ParseSRI parses an SRI hash string and returns the algorithm and hash bytes.
func ParseSRI(sri string) (algorithm string, hash []byte, err error) {
	parts := strings.SplitN(sri, "-", 2)
	if len(parts) != 2 {
		return "", nil, fmt.Errorf("invalid SRI format: %s", sri)
	}

	algorithm = parts[0]
	hashStr := parts[1]

	// Try base64 first
	hash, err = base64.StdEncoding.DecodeString(hashStr)
	if err != nil {
		// Try hex
		hash, err = hex.DecodeString(hashStr)
		if err != nil {
			return "", nil, fmt.Errorf("decoding hash: %w", err)
		}
	}

	return algorithm, hash, nil
}

// ValidateSRI checks if an SRI hash string is valid.
func ValidateSRI(sri string) error {
	algo, hash, err := ParseSRI(sri)
	if err != nil {
		return err
	}

	switch algo {
	case "sha256":
		if len(hash) != 32 {
			return fmt.Errorf("sha256 hash must be 32 bytes, got %d", len(hash))
		}
	case "sha512":
		if len(hash) != 64 {
			return fmt.Errorf("sha512 hash must be 64 bytes, got %d", len(hash))
		}
	default:
		return fmt.Errorf("unsupported algorithm: %s", algo)
	}

	return nil
}
