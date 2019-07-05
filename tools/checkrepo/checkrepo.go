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
	"strings"
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
	logFilter    = flag.String("log-filter", "", `Message types to filter: [E]rrors, [N]on-updates, [U]pdates`)
	concurrency  = flag.Int("concurrency", 16, "Number of concurrent workers")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <url>...   -- process these URLs
       %[1]s [options] -stdin     -- process URLs from stdin, one per line
       %[1]s [options] -scan      -- process URLs from a dependency graph (requires -store)

Scan the specified Git repository URLs to see whether any have changed since
the last update recorded in the database. Use -polldb to specify the path of
the database, or set REPODEPS_POLLDB in the environment.

By default %[1]s processes URLs named on the command line. Use -stdin to
additionally read URLs from stdin, one per line. Use -scan to read URLs from an
existing dependency graph database (-store).

Updates are scheduled not less than -interval apart, and scale to the frequency
of observed changes in the repository. Of the eligible URLs, a random fraction
are selected to be polled via the -sample flag. Use -sample 1 to select all.

For each URL examined, %[1]s writes a JSON text to stdout:

  {
    "repository":  "url",  // the fetch URL of the repository
    "needsUpdate": true,   // reports whether an update is neede
    "reference":   "ref",  // the name of the reference used for comparison
    "digest":      "xxx",  // the SHA-1 digest at that reference
    "errors":      1       // number of consecutive check failures
  }

If a repository was previously unrecorded, it is added to the database and
reported as needing an update.  If the errors count exceeds -error-limit, the
repository is removed from the database. Otherwise, if the repository has
changed since the last check it is reported as needing an update.

If -clone is set, each repository reported as needing an update will be fetched
and checked out at the new commit position in a subdirectory of -clone-dir.  In
this case a "clone" field is added to the output naming the directory.

If -update is set, each repository reported as needing an update will be
fetched and checked out at the new commit position, scanned for new dependency
information, and written to the specified -store. If -clone is also set, these
clones are retained when the program exits; otherwise they are deleted once the
update is complete. If -force is true, -update will update all eligible repos,
even those which have not changed.

Up to -concurrency repositories may be concurrently processed.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Check command-line flags.
	*logFilter = strings.ToUpper(*logFilter)
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
				Ref:  res.Name,
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
			numUpdates += out.Pkgs
			if pickLog(out) {
				omu.Lock()
				enc.Encode(out)
				omu.Unlock()
			}
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
`, numURL, numDups, numSamples, numUpdates, time.Since(start).Truncate(time.Millisecond))
}

type result struct {
	Need  bool   `json:"needsUpdate"`
	Repo  string `json:"repository"`
	Ref   string `json:"reference"`
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

func pickLog(msg *result) bool {
	if msg.Errs != 0 {
		return strings.IndexByte(*logFilter, 'E') < 0
	} else if msg.Need {
		return strings.IndexByte(*logFilter, 'U') < 0
	}
	return strings.IndexByte(*logFilter, 'N') < 0
}
