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

// Program checkrepo checks the current status of one or more repositories
// against a database of known latest digests.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

var (
	pollDBPath  = flag.String("polldb", "", "Poll database path (required)")
	storePath   = flag.String("store", "", "Storage database path (required with -update)")
	cloneDir    = flag.String("clone-dir", "", `Location to store clones ("" uses $TMPDIR)`)
	doReadStdin = flag.Bool("stdin", false, "Read repo URLs from stdin")
	doClone     = flag.Bool("clone", false, "Clone updated repositories")
	doUpdate    = flag.Bool("update", false, "Update cloned repositories (implies -clone)")
	concurrency = flag.Int("concurrency", 16, "Number of concurrent workers")
)

func main() {
	flag.Parse()

	// Check command-line flags.
	if *storePath == "" && *doUpdate {
		log.Fatal("You must specify a non-empty -store in order to -update")
	} else if (*doClone || *doUpdate) && *cloneDir == "" {
		tmp, err := ioutil.TempDir("", "checkrepo-")
		if err != nil {
			log.Fatalf("Creating temp directory: %v", err)
		}
		*cloneDir = tmp
	}

	// Open the poll state database.
	db, c, err := tools.OpenPollDB(*pollDBPath, tools.ReadWrite)
	if err != nil {
		log.Fatalf("Opening poll database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up an updater; by default a no-op.
	update, cleanup := updater(ctx)
	defer func() {
		if err := cleanup(); err != nil {
			log.Fatalf("Closing graph: %v", err)
		}
	}()

	g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(*concurrency)

	var omu sync.Mutex // guards stdout, numUpdates
	var numUpdates int
	seen := stringset.New()
	enc := json.NewEncoder(os.Stdout)

	for url := range tools.Inputs(*doReadStdin) {
		url := url
		if seen.Contains(url) {
			log.Printf("[skipped] duplicate URL %q", url)
			continue
		}
		seen.Add(url)
		run(func() error {
			res, err := db.Check(ctx, url)
			if err != nil {
				log.Printf("[skipped] checking %q: %v", url, err)
				return nil
			}
			out := &result{
				Need: res.NeedsUpdate(),
				Repo: res.URL,
				Name: res.Name,
				Hex:  res.Digest,
			}
			if *doClone || *doUpdate {
				path := filepath.Join(*cloneDir, res.Digest)
				if err := res.Clone(ctx, path); err != nil {
					log.Printf("[skipped] cloning %q failed: %v", res.URL, err)
					return nil
				}
				out.Clone = path
				if *doUpdate && !*doClone {
					defer os.RemoveAll(path) // clean up after update, if -clone is not set
				}
				n, err := update(path)
				if err != nil {
					log.Printf("[skipped] updating %q failed: %v", res.URL, err)
					return nil
				}
				out.Pkgs = n
			}
			omu.Lock()
			numUpdates += out.Pkgs
			enc.Encode(out)
			omu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatalf("Processing failed: %v", err)
	}
	if err := c.Close(); err != nil {
		log.Fatalf("Closing storage: %v", err)
	}
	if *doUpdate {
		log.Printf("Updated %d packages total", numUpdates)
	}
}

type result struct {
	Need  bool   `json:"update"`
	Repo  string `json:"repository"`
	Name  string `json:"name"`
	Hex   string `json:"digest"`
	Clone string `json:"clone,omitempty"`
	Pkgs  int    `json:"packages"`
}

func updater(ctx context.Context) (func(path string) (int, error), func() error) {
	if !*doUpdate {
		return func(string) (int, error) { return 0, nil }, func() error { return nil }
	}
	g, c, err := tools.OpenGraph(*storePath, tools.ReadWrite)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	cleanup := func() error { return c.Close() }
	return func(path string) (int, error) {
		repos, err := local.Load(ctx, path, &deps.Options{
			HashSourceFiles:   true,
			UseImportComments: true,
		})
		if err != nil {
			return 0, err
		}
		var added int
		for _, repo := range repos {
			if err := g.AddAll(ctx, repo); err != nil {
				return 0, err
			}
			added += len(repo.Packages)
		}
		return added, nil
	}, cleanup
}
