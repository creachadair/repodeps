package deps

import (
	"crypto/sha256"
	"io"
	"strings"
)

//go:generate protoc --go_out=. deps.proto

// Options control the behaviour of the Load function. A nil *Options behaves
// as a zero-valued Options struct.
type Options struct {
	HashSourceFiles bool // record source file digests
}

// Hash produces a SHA-256 digest of the contents of r.
func Hash(r io.Reader) []byte {
	h := sha256.New()
	io.Copy(h, r)
	return h.Sum(nil)
}

// IsVendor reports whether the specified path is in a vendor/ directory.
func IsVendor(path string) bool {
	return strings.Contains(path, "/vendor/") || strings.HasPrefix(path, "vendor/")
}
