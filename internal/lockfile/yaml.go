package lockfile

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultLockfile is the default lockfile name.
	DefaultLockfile = "nopher.lock.yaml"
)

// Load reads a lockfile from the given path.
func Load(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading lockfile: %w", err)
	}

	var lf Lockfile
	if err := yaml.Unmarshal(data, &lf); err != nil {
		return nil, fmt.Errorf("parsing lockfile: %w", err)
	}

	return &lf, nil
}

// Save writes the lockfile in YAML format.
func (lf *Lockfile) Save(dir string) error {
	return lf.SaveYAML(filepath.Join(dir, DefaultLockfile))
}

// SaveYAML writes the lockfile in YAML format.
func (lf *Lockfile) SaveYAML(path string) error {
	data, err := yaml.Marshal(lf)
	if err != nil {
		return fmt.Errorf("marshaling YAML: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing lockfile: %w", err)
	}

	return nil
}
