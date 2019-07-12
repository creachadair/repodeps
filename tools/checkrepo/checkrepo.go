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
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/tools"
)

var (
	repoDBPath   = flag.String("repo-db", os.Getenv("REPODEPS_POLLDB"), "Poll database path (required)")
	graphDBPath  = flag.String("graph-db", "", "Storage database path (required with -update)")
	cloneDir     = flag.String("clone-dir", "", `Location to store clones ("" uses $TMPDIR)`)
	doForce      = flag.Bool("force", false, "Force update of matching repositories")
	doReadStdin  = flag.Bool("stdin", false, "Read repo URLs from stdin")
	doScanDB     = flag.Bool("scan", false, "Read repo URLs from the poll database")
	doUpdate     = flag.Bool("update", false, "Update cloned repositories")
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
the last update recorded in the database. Use -repo-db to specify the path of
the database, or set REPODEPS_POLLDB in the environment.

By default %[1]s processes URLs named on the command line. Use -stdin to
additionally read URLs from stdin, one per line. Use -scan to read URLs from an
existing dependency graph database (-graph-db).

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

If -update is set, each repository reported as needing an update will be
fetched and checked out at the new commit position, scanned for new dependency
information, and written to the specified -graph-db. These clones are deleted
once the update is complete.  If -force is true, -update will update all
eligible repos, even those which have not changed.

Up to -concurrency repositories may be concurrently processed.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	enc := json.NewEncoder(os.Stdout) // log writer
	opts := service.Options{
		RepoDB:  *repoDBPath,
		GraphDB: *graphDBPath,
		WorkDir: *cloneDir,

		// Scan options.
		MinPollInterval: *pollInterval,
		ErrorLimit:      *errorLimit,
		SampleRate:      *sampleRate,
		Concurrency:     *concurrency,

		// Default package loader options.
		Options: deps.Options{
			HashSourceFiles:   true,
			UseImportComments: true,
		},

		// Stream logs to stdout.
		StreamLog: func(_ context.Context, key string, arg interface{}) error {
			return enc.Encode([]interface{}{time.Now().In(time.UTC), key, arg})
		},
	}
	*logFilter = strings.ToUpper(*logFilter)

	u, err := service.New(opts)
	if err != nil {
		log.Fatalf("Creating service: %v", err)
	}
	defer func() {
		if err := u.Close(); err != nil {
			log.Fatalf("Closing service: %v", err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if *doScanDB {
		log.Printf("--- BEGIN update scan (sample fraction %f)", *sampleRate)
		start := time.Now()
		rsp, err := u.Scan(ctx, &service.ScanReq{
			LogUpdates:    strings.IndexByte(*logFilter, 'U') < 0,
			LogErrors:     strings.IndexByte(*logFilter, 'E') < 0,
			LogNonUpdates: strings.IndexByte(*logFilter, 'N') < 0,
		})
		if err != nil {
			log.Printf("Scan failed: %v", err)
		} else {
			enc.Encode(rsp)
		}
		log.Printf(`Processing complete:
%-6d URLs scanned
%-6d duplicates discarded
%-6d samples selected
%-6d packages updated in %d repositories

--- DONE [%v elapsed]`, rsp.NumScanned, rsp.NumDups, rsp.NumSamples,
			rsp.NumPackages, rsp.NumUpdates, time.Since(start))
		return
	}
	for url := range tools.Inputs(*doReadStdin) {
		rsp, err := u.Update(ctx, &service.UpdateReq{
			Repository: url,
			CheckOnly:  !*doUpdate,
			Force:      *doForce,
		})
		if err != nil {
			log.Printf("Update %q failed: %v", url, err)
		} else {
			enc.Encode(rsp)
		}
	}
}
