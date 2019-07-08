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

// Package tools implements shared code for command-line tools.
package tools

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
)

// OpenMode controls how OpenGraph accesses storage.
type OpenMode int

// Mode constants for OpenGraph.
const (
	ReadOnly OpenMode = iota
	ReadWrite
)

// OpenGraph opens the graph named by path.  The caller must ensure the closer
// is closed when the graph is no longer in use.
func OpenGraph(path string, mode OpenMode) (*graph.Graph, io.Closer, error) {
	s, err := openBadger(path, mode)
	if err != nil {
		return nil, nil, fmt.Errorf("opening storage: %v", err)
	}
	return graph.New(storage.NewBlob(s)), s, nil
}

// OpenPollDB opens the poll database named by path. The caller must ensure the
// closer is closed when the graph is no longer in use.
func OpenPollDB(path string, mode OpenMode) (*poll.DB, io.Closer, error) {
	s, err := openBadger(path, mode)
	if err != nil {
		return nil, nil, fmt.Errorf("opening storage: %v", err)
	}
	return poll.NewDB(storage.NewBlob(s)), s, nil
}

func openBadger(path string, mode OpenMode) (*badgerstore.Store, error) {
	if path == "" {
		return nil, errors.New("no path was provided")
	}
	if mode == ReadWrite {
		return badgerstore.NewPath(path)
	}
	return badgerstore.NewPathReadOnly(path)
}

// Inputs returns a channel that delivers the paths of inputs and is closed
// when no more are available. The non-flag arguments are read, and if
// readStdin is true each line of stdin is also read. The caller must fully
// drain the channel.
func Inputs(readStdin bool) <-chan string {
	ch := make(chan string, len(flag.Args()))
	for _, arg := range flag.Args() {
		ch <- arg
	}
	if readStdin {
		s := bufio.NewScanner(os.Stdin)
		go func() {
			defer close(ch)
			for s.Scan() {
				ch <- s.Text()
			}
		}()
	} else {
		close(ch)
	}
	return ch
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

// ScanDB returns a channel that delivers all the keys of db and is then
// closed. The caller must ensure the channel is fully drained. If ctx
// completes the channel will be closed automatically.
func ScanDB(ctx context.Context, db *poll.DB, min time.Duration) <-chan string {
	ch := make(chan string)
	go func() {
		defer close(ch)
		if err := db.Scan(ctx, "", func(url string) error {
			stat, err := db.Status(ctx, url)
			if err != nil {
				return err
			} else if poll.ShouldCheck(stat, min) {
				ch <- url
			}
			return nil
		}); err != nil {
			log.Printf("Warning: scanning failed: %v", err)
		}
	}()
	return ch
}

// NewMatcher constructs a function that reports true for names that match one
// of the specified patterns. Each pattern is either a plain string, which
// matches exactly, or a prefix match ending with "/...".
func NewMatcher(pats []string) func(string) bool {
	var ps []string
	for _, pat := range pats {
		if t := strings.TrimSuffix(pat, "/..."); t != pat {
			ps = append(ps, regexp.QuoteMeta(t)+"(?:/.*)?$")
		} else {
			ps = append(ps, regexp.QuoteMeta(pat)+"$")
		}
	}
	re := regexp.MustCompile("^(?:" + strings.Join(ps, "|") + ")")
	return re.MatchString
}
