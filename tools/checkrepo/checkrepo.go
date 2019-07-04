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
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

var (
	pollDBPath   = flag.String("polldb", os.Getenv("REPODEPS_POLLDB"), "Poll database path (required)")
	storePath    = flag.String("store", "", "Storage database path (required with -update)")
	cloneDir     = flag.String("clone-dir", "", `Location to store clones ("" uses $TMPDIR)`)
	doForce      = flag.Bool("force", false, "Force update of matching repositories")
	doReadStdin  = flag.Bool("stdin", false, "Read repo URLs from stdin")
	doScanDB     = flag.Bool("scan", false, "Read repo URLs from the poll database")
	doClone      = flag.Bool("clone", false, "Clone updated repositories")
	doUpdate     = flag.Bool("update", false, "Update cloned repositories (implies -clone)")
	errorLimit   = flag.Int("error-limit", 10, "Discard repositories that fail more than this many times")
	pollInterval = flag.Duration("interval", 1*time.Hour, "Minimum polling interval")
	sampleRate   = flag.Float64("sample", 1, "Sample this fraction of eligible updates (0..1)")
	concurrency  = flag.Int("concurrency", 16, "Number of concurrent workers")
)

func main() {
	flag.Parse()

	// Check command-line flags.
	if *storePath == "" && *doUpdate {
		log.Fatal("You must specify a non-empty -store in order to -update")
	} else if *sampleRate < 0 || *sampleRate > 1 {
		log.Fatalf("Sample rate %f is out of range (0..1)", *sampleRate)
	} else if (*doClone || *doUpdate) && *cloneDir == "" {
		tmp, err := ioutil.TempDir("", "checkrepo-")
		if err != nil {
			log.Fatalf("Creating temp directory: %v", err)
		}
		defer os.Remove(tmp) // best-effort cleanup if empty at exit
		*cloneDir = tmp
	}

	// Open the poll state database.
	db, c, err := tools.OpenPollDB(*pollDBPath, tools.ReadWrite)
	if err != nil {
		log.Fatalf("Opening poll database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var urls <-chan string
	if *doScanDB {
		urls = tools.ScanDB(ctx, db, *pollInterval)
	} else {
		urls = tools.Inputs(*doReadStdin)
	}

	// Set up an updater; by default a no-op.
	update, cleanup := updater(ctx)
	defer func() {
		if err := cleanup(); err != nil {
			log.Fatalf("Closing graph: %v", err)
		}
	}()

	g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(*concurrency)

	var omu sync.Mutex // guards stdout, numUpdates
	var numURL, numSamples, numUpdates, numDups int
	seen := stringset.New()
	enc := json.NewEncoder(os.Stdout)

	start := time.Now()
	for url := range urls {
		numURL++
		url := tools.FixRepoURL(url)
		if seen.Contains(url) {
			numDups++
			continue
		} else if !pickSample(url) {
			continue
		}
		numSamples++
		seen.Add(url)
		run(func() error {
			res, err := db.Check(ctx, url)
			if err != nil && res == nil { // structural failure
				log.Printf("[skipped] checking %q: %v", url, err)
				return nil
			}
			if res.Errors > *errorLimit { // update failure
				db.Remove(ctx, url)
				log.Printf("Removed %q after %d failures", url, res.Errors)
			}
			out := &result{
				Need: res.NeedsUpdate(),
				Repo: res.URL,
				Name: res.Name,
				Hex:  res.Digest,
				Errs: res.Errors,
			}
			if (res.NeedsUpdate() || *doForce) && (*doClone || *doUpdate) {
				path := filepath.Join(*cloneDir, fmt.Sprintf("%s.%p", res.Digest, res))
				if err := res.Clone(ctx, path); err != nil {
					log.Printf("[skipped] cloning %q failed: %v", res.URL, err)
					return nil
				}
				if *doUpdate && !*doClone {
					defer os.RemoveAll(path) // clean up after update, if -clone is not set
				} else {
					out.Clone = path
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
	log.Printf(`Processing complete:
%-6d URLs scanned
%-6d duplicates discarded
%-6d samples selected
%-6d packages updated

[%v elapsed]
`, numURL, numDups, numSamples, numUpdates, time.Since(start))
}

type result struct {
	Need  bool   `json:"needsUpdate"`
	Repo  string `json:"repository"`
	Name  string `json:"name"`
	Hex   string `json:"digest,omitempty"`
	Clone string `json:"clone,omitempty"`
	Pkgs  int    `json:"numPackages,omitempty"`
	Errs  int    `json:"errors,omitempty"`
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

func pickSample(_ string) bool { return rand.Float64() < *sampleRate }
