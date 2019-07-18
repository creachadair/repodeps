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

// Package poll defines an interface for periodically polling repositories for
// status updates.
package poll

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/creachadair/repodeps/storage"
	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes"
)

//go:generate protoc --go_out=. poll.proto

// A DB represents a cache of update statuses for repositories.
type DB struct {
	st storage.Interface
}

// NewDB constructs a database handle for the given storage.
func NewDB(st storage.Interface) *DB { return &DB{st: st} }

// Status returns the status record for the specified URL.  It is an error if
// the given URL does not have a record in this database.
func (db *DB) Status(ctx context.Context, url string) (*Status, error) {
	var stat Status
	if err := db.st.Load(ctx, url, &stat); err != nil {
		return nil, err
	}
	return &stat, nil
}

// Remove removes the status record for the specified URL.
func (db *DB) Remove(ctx context.Context, url string) error {
	return db.st.Delete(ctx, url)
}

// Scan scans all the URLs in the database. If f reports an error, that error
// is propagated to the caller.
func (db *DB) Scan(ctx context.Context, f func(string) error) error {
	return db.st.Scan(ctx, "", f)
}

// CheckOptions control optional features of repostory checks.  A nil
// *CheckOptions is ready for use with default values.
type CheckOptions struct {
	// If set, use this reference name to resolve a target digest.
	Reference string

	// If set, attribute this prefix to the packages found in the repository.
	Prefix string
}

func (o *CheckOptions) refName() string {
	if o != nil && o.Reference != "" {
		return o.Reference
	}
	return "*"
}

func (o *CheckOptions) prefix() string {
	if o == nil {
		return ""
	}
	return o.Prefix
}

// Check reports whether the specified repository requires an update. If the
// repository does not exist, it is added and reported as needing update.
//
// If there is an error in updating the status, the check result will be
// non-nil and the caller can check the Errors field to see how often an update
// has been attempted without success. Repositories that fail too often may be
// pruned from the database.
//
// If url has the form <base>@@<tag>, the specified tag is applied.
func (db *DB) Check(ctx context.Context, url string, opts *CheckOptions) (*CheckResult, error) {
	url, tag := url, opts.refName()
	stat, err := db.Status(ctx, url)
	if err == storage.ErrKeyNotFound {
		// This is a new repository; set up the initial state.
		stat = &Status{
			Repository: url,
			RefName:    tag, // to be updated
			Prefix:     opts.prefix(),
		}
	} else if err != nil {
		return nil, err
	}

	// If the reference name has changed, force an update.
	if tag != "*" && tag != stat.RefName {
		stat.RefName = tag
		stat.Digest = nil
	}

	// If a prefix was provided, record it.
	if pfx := opts.prefix(); pfx != "" {
		stat.Prefix = pfx
	}

	// Build the return value before updating the saved state.
	old := hex.EncodeToString(stat.Digest)
	st := &CheckResult{
		URL:    url,
		Name:   stat.RefName,
		Digest: old,
		Prefix: stat.Prefix,
		old:    old,
	}

	// Try to update the repository state. If this fails, report the partial
	// results back to the caller.
	name, digest, err := bestHead(ctx, url, stat.RefName)
	if err != nil {
		stat.ErrorCount++
		st.Errors = int(stat.ErrorCount)
		db.st.Store(ctx, url, stat)
		return st, err
	}
	st.Name = name
	st.Digest = hex.EncodeToString(digest)

	// If this isn't the first update, save the current value as history.
	if len(stat.Digest) != 0 && st.NeedsUpdate() {
		stat.Updates = append(stat.Updates, &Status_Update{
			When:   stat.LastCheck,
			Digest: stat.Digest,
		})

		if n := len(stat.Updates); n > 20 {
			stat.Updates = stat.Updates[n-20:]
		}
	}
	stat.RefName = name
	stat.Digest = digest
	stat.LastCheck = ptypes.TimestampNow()
	stat.ErrorCount = 0 // success resets the counter

	// Write the new state back to storage.
	if err := db.st.Store(ctx, url, stat); err != nil {
		return nil, err
	}
	return st, nil
}

// RepoExists reports whether there is a repository at the specified URL.
//
// A true result means the repository is definitely present, though it may
// require authentication. A false result may be incorrect, for example if the
// caller's network filters outbound git requests.
func RepoExists(ctx context.Context, url string) bool {
	_, err := git(ctx, "ls-remote", "-q", url, "").Output()
	if err == nil {
		return true
	} else if e, ok := err.(*exec.ExitError); ok {
		line := strings.Split(strings.TrimSpace(string(e.Stderr)), ":")
		msg := strings.TrimSpace(strings.ToLower(line[len(line)-1]))
		return msg == "terminal prompts disabled" // exists, but wants authentication
	}
	return false
}

func bestHead(ctx context.Context, url, ref string) (name string, digest []byte, _ error) {
	out, err := git(ctx, "ls-remote", "-q", url, ref).Output()
	if err != nil {
		return "", nil, runErr(err)
	}
	var hexDigest string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue // wrong form
		}
		if parts[1] == "refs/heads/master" {
			name, hexDigest = parts[1], parts[0] // master is preferred, if present
			break
		} else if !isInterestingRef(parts[1], ref != "") {
			continue // not interesting
		}

		// Take the first available candidate, falling back to HEAD.
		if name == "" || name == "HEAD" {
			name, hexDigest = parts[1], parts[0]
		}
	}
	if name == "" {
		return "", nil, errors.New("no matching remote heads")
	} else if digest, err = hex.DecodeString(hexDigest); err != nil {
		return "", nil, fmt.Errorf("invalid digest: %v", err)
	}
	return
}

func isInterestingRef(ref string, tagsOK bool) bool {
	switch {
	case strings.HasPrefix(ref, "refs/heads/"):
		return true
	case tagsOK && strings.HasPrefix(ref, "refs/tags/"):
		return true
	case ref == "HEAD":
		return true
	}
	return false
}

func git(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	return cmd
}

func runErr(err error) error {
	if e, ok := err.(*exec.ExitError); ok {
		line := strings.Join(strings.Split(string(e.Stderr), "\n"), " ")
		return errors.New(line)
	}

	return err
}

// MarshalJSON implements json.Marshaler for a Status by delegating to jsonpb.
func (s *Status) MarshalJSON() ([]byte, error) {
	// It is manifestly ridiculous that we have to do this.

	var enc jsonpb.Marshaler
	t, err := enc.MarshalToString(s)
	if err != nil {
		return nil, err
	}
	return []byte(t), nil
}

// UnmarshalJSON implements json.Unmarshaler for a Status by delegating to jsonpb.
func (s *Status) UnmarshalJSON(data []byte) error {
	var dec jsonpb.Unmarshaler
	return dec.Unmarshal(bytes.NewReader(data), s)
}

// FixRepoURL ensures s has a valid protocol prefix for Git.
func FixRepoURL(s string) string {
	return "https://" + CleanRepoURL(s)
}

// CleanRepoURL removes protocol and format tags from a repository URL.
func CleanRepoURL(url string) string {
	if trim := strings.TrimPrefix(url, "git@"); trim != url {
		parts := strings.SplitN(trim, ":", 2)
		url = parts[0]
		if len(parts) == 2 {
			url += "/" + parts[1]
		}
	} else if parts := strings.SplitN(url, "://", 2); len(parts) == 2 {
		url = parts[1] // discard http:// or https://
	}
	return strings.TrimSuffix(url, ".git")
}
