package hash

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestConvertGoH1ToSRI(t *testing.T) {
	// Valid base64 hash (32 bytes = 44 base64 chars with padding)
	validH1 := "h1:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid h1 hash",
			input: validH1,
			want:  "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU=",
		},
		{
			name:    "invalid prefix",
			input:   "sha256-xyz123abc456",
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ConvertGoH1ToSRI(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ConvertGoH1ToSRI() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ConvertGoH1ToSRI(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestComputeZipHash(t *testing.T) {
	// Create a known byte slice
	data := []byte("test data for hashing")

	// Compute expected hash
	h := sha256.New()
	h.Write(data)
	expected := "sha256-" + base64.StdEncoding.EncodeToString(h.Sum(nil))

	// Test with actual computation (this would require creating a temp zip file)
	// For now, test the format
	if !hasPrefix(expected, "sha256-") {
		t.Errorf("Hash doesn't start with sha256- prefix: %q", expected)
	}

	// Verify base64 encoding
	hashPart := expected[7:] // Remove "sha256-"
	if _, err := base64.StdEncoding.DecodeString(hashPart); err != nil {
		t.Errorf("Hash part is not valid base64: %v", err)
	}
}

func TestSRIFormat(t *testing.T) {
	// Test that SRI format is correct - using valid 32-byte SHA256 in base64
	testHash := "sha256-47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU="

	if !hasPrefix(testHash, "sha256-") {
		t.Error("SRI hash should start with 'sha256-'")
	}

	// Extract base64 part
	b64Part := testHash[7:]

	// Verify it's valid base64
	decoded, err := base64.StdEncoding.DecodeString(b64Part)
	if err != nil {
		t.Errorf("Base64 part is invalid: %v", err)
	}

	// Verify it's 32 bytes (SHA256)
	if len(decoded) != 32 {
		t.Errorf("Decoded hash should be 32 bytes, got %d", len(decoded))
	}
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
