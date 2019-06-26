package deps

import (
	"crypto/sha256"
	"io"
	"strings"
)

// Options control the behaviour of the Load function. A nil *Options behaves
// as a zero-valued Options struct.
type Options struct {
	HashSourceFiles bool // record source file digests
}

// A Repo records the Go package structure of a repository.
type Repo struct {
	From     string     `json:"from,omitempty"` // file or directory, cosmetic
	Remotes  []*Remote  `json:"remotes,omitempty"`
	Packages []*Package `json:"packages,omitempty"`
}

// A Package records the name and dependencies of a single package.
type Package struct {
	Name       string   `json:"name"`
	ImportPath string   `json:"importPath"`
	Imports    []string `json:"imports,omitempty"`
	Source     []*File  `json:"source,omitempty"`
}

// A Remote records the name and URL of a Git remote.
type Remote struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// A File records the name and content digest of a file.  The name of the file
// is recorded as a path relative to the root of the repository.
type File struct {
	Name   string `json:"name"`
	Digest []byte `json:"sha256"`
}

// Hash produces a SHA-256 digest of the contents of r.
func Hash(r io.Reader) []byte {
	h := sha256.New()
	io.Copy(h, r)
	return h.Sum(nil)
}

// IsVendor reports whether the specified path is in a vendor/ directory.
func IsVendor(path string) bool { return strings.Contains(path, "/vendor/") }
