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
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/creachadair/repodeps/graph"
	"github.com/golang/protobuf/ptypes"
)

//go:generate protoc --go_out=. poll.proto

// A DB represents a cache of update statuses for repositories.
type DB struct {
	st graph.Storage
}

// NewDB constructs a database handle for the given storage.
func NewDB(st graph.Storage) *DB { return &DB{st: st} }

// CheckResult records the update status of a repository.
type CheckResult struct {
	URL    string // repository fetch URL
	Name   string // remote head name
	Digest string // current digest value

	old string // old digest value
}

// NeedsUpdate reports whether c requires an update.
func (c *CheckResult) NeedsUpdate() bool { return c.old != c.Digest }

// Status returns the status record for the specified URL.  It is an error if
// the given URl does not have a record in this database.
func (db *DB) Status(ctx context.Context, url string) (*Status, error) {
	var stat Status
	if err := db.st.Load(ctx, url, &stat); err != nil {
		return nil, err
	}
	return &stat, nil
}

// Check reports whether the specified repository requires an update. If the
// repository does not exist, it is added and reported as needing update.
func (db *DB) Check(ctx context.Context, url string) (*CheckResult, error) {
	stat, err := db.Status(ctx, url)
	if err == graph.ErrKeyNotFound {
		// This is a new repository; set up the initial state.
		stat = &Status{
			Repository: url,
			RefName:    "refs/heads/*",
		}
	} else if err != nil {
		return nil, err
	}
	name, digest, err := firstHead(ctx, url, stat.RefName)
	if err != nil {
		return nil, err
	}
	dec, err := hex.DecodeString(digest)
	if err != nil {
		return nil, fmt.Errorf("invalid digest: %v", err)
	}

	// Build the return value before updating the saved state.
	st := &CheckResult{
		URL:    url,
		Name:   name,
		Digest: digest,

		old: hex.EncodeToString(stat.Digest),
	}

	// If this isn't the first update, save the current value as history.
	if len(stat.Digest) != 0 && st.NeedsUpdate() {
		stat.Updates = append(stat.Updates, &Status_Update{
			When:   stat.LastCheck,
			Digest: stat.Digest,
		})
	}
	stat.RefName = name
	stat.Digest = dec
	stat.LastCheck = ptypes.TimestampNow()

	// Write the new state back to storage.
	if err := db.st.Store(ctx, url, stat); err != nil {
		return nil, err
	}
	return st, nil
}

func firstHead(ctx context.Context, url, ref string) (name, digest string, _ error) {
	cmd := exec.CommandContext(ctx, "git", "ls-remote", "-q", url, ref)
	out, err := cmd.Output()
	if err != nil {
		return "", "", err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 {
			return parts[1], parts[0], nil
		}
	}
	return "", "", errors.New("no remote heads")
}