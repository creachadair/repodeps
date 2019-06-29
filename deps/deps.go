// Copyright 2019 Michael J. Fromberger. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package deps defines data structures and functions for recording the
// dependencies between packages.
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
	HashSourceFiles   bool // record source file digests
	UseImportComments bool // use import comments to name packages
}

// Hash produces a SHA-256 digest of the contents of r.
func Hash(r io.Reader) []byte {
	h := sha256.New()
	io.Copy(h, r)
	return h.Sum(nil)
}

// IsVendor reports whether the specified path is in a vendor/ directory.
func IsVendor(path string) bool {
	return strings.HasPrefix(path, "vendor/") || strings.Contains(path, "/vendor/")
}
