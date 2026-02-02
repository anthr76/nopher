// Package lockfile provides types and functions for working with nopher lockfiles.
package lockfile

// Schema version for the lockfile format.
const SchemaVersion = 1

// Lockfile represents the nopher.lock.yaml file structure.
type Lockfile struct {
	Schema  int                `json:"schema" yaml:"schema"`
	Go      string             `json:"go" yaml:"go"`
	Modules map[string]Module  `json:"modules,omitempty" yaml:"modules,omitempty"`
	Replace map[string]Replace `json:"replace,omitempty" yaml:"replace,omitempty"`
}

// Module represents a single Go module dependency.
type Module struct {
	Version string `json:"version" yaml:"version"`
	Hash    string `json:"hash" yaml:"hash"`
	URL     string `json:"url,omitempty" yaml:"url,omitempty"`
	Rev     string `json:"rev,omitempty" yaml:"rev,omitempty"`
}

// Replace represents a module replacement directive.
type Replace struct {
	// For remote replacements
	Old        string `json:"old,omitempty" yaml:"old,omitempty"`               // Original module path (usually same as key)
	OldVersion string `json:"oldVersion,omitempty" yaml:"oldVersion,omitempty"` // Original version from go.mod
	New        string `json:"new,omitempty" yaml:"new,omitempty"`
	Version    string `json:"version,omitempty" yaml:"version,omitempty"` // New version
	Hash       string `json:"hash,omitempty" yaml:"hash,omitempty"`
	URL        string `json:"url,omitempty" yaml:"url,omitempty"`
	Rev        string `json:"rev,omitempty" yaml:"rev,omitempty"`

	// For local replacements
	Path string `json:"path,omitempty" yaml:"path,omitempty"`
}

// New creates a new Lockfile with the given Go version.
func New(goVersion string) *Lockfile {
	return &Lockfile{
		Schema:  SchemaVersion,
		Go:      goVersion,
		Modules: make(map[string]Module),
		Replace: make(map[string]Replace),
	}
}
