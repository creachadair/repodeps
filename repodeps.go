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

// Program repodeps scans the contents of a collection of GitHub repositories
// and reports the names and dependencies of any Go packages defined inside.
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/storage"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
	"github.com/golang/protobuf/jsonpb"
)

var (
	storePath     = flag.String("store", "", "Storage path")
	doReadStdin   = flag.Bool("stdin", false, "Read input filenames from stdin")
	doSourceHash  = flag.Bool("sourcehash", true, "Record the names and digests of source files")
	doImportComm  = flag.Bool("import-comments", true, "Parse and use import comments")
	doTrimRepo    = flag.Bool("trim-repo", false, "Trim the repository prefix from each import path")
	doStandardLib = flag.Bool("stdlib", false, "Treat packages in the input as standard libraries")
	pkgPrefix     = flag.String("prefix", "", "Attribute package names to this prefix")
	taskTimeout   = flag.Duration("timeout", 5*time.Minute, "Timeout on processing a single repository")
	concurrency   = flag.Int("concurrency", 32, "Maximum concurrent workers")

	out = &struct {
		sync.Mutex
		io.Writer
	}{Writer: os.Stdout}
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <path>...

Search the specified Git repositories for Go source packages, and record the
names and package dependencies of each package found. Each non-flag argument
should be be a Git directory path.

If -store is set, output is written to a database at that path; otherwise
output is streamed to stdout as JSON.

If -stdin is set, then each line of stdin is read after all the non-flag
arguments are processed.

If -sourcehash is set, the repository-relative paths and content digests of the
Go source file in each packge are also captured.

Inputs are processed concurrently with up to -concurrency in parallel.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 && !*doReadStdin {
		log.Fatalf("Usage: %s <repo-dir> ...", filepath.Base(os.Args[0]))
	}
	ctx, cancel := context.WithCancel(context.Background())
	opts := &deps.Options{
		HashSourceFiles:   *doSourceHash,
		UseImportComments: *doImportComm,
		TrimRepoPrefix:    *doTrimRepo,
		StandardLibrary:   *doStandardLib,
		PackagePrefix:     *pkgPrefix,
	}
	defer cancel()
	var db *graph.Graph
	if *storePath != "" {
		s, err := badgerstore.NewPath(*storePath)
		if err != nil {
			log.Fatalf("Opening graph: %v", err)
		}
		db = graph.New(storage.NewBlob(s))
		defer func() {
			if err := s.Close(); err != nil {
				log.Fatalf("Closing graph: %v", err)
			}
		}()
		log.Printf("Writing output to graph %q", *storePath)
	}

	g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(*concurrency)

	// Each argument is a git repository path.
	var numRepos int
	start := time.Now()
	for dir := range tools.Inputs(*doReadStdin) {
		dir := dir
		path, err := filepath.Abs(dir)
		if err != nil {
			log.Fatalf("Resolving path: %v", err)
		}
		numRepos++
		run(func() error {
			// Each repository or archive has a separate task timeout to ensure
			// pathological repos do not choke the tool. We start timing once the
			// goroutine runs so that wait time doesn't count against the task.
			tctx, cancel := context.WithTimeout(ctx, *taskTimeout)
			defer cancel()
			log.Printf("Processing %q...", dir)

			repos, err := local.Load(tctx, path, opts)
			if err != nil {
				log.Printf("Skipped %q:\n  %v", dir, err)
				return nil
			}

			if db != nil {
				return writeReposDB(ctx, db, repos)
			}
			return writeReposJSON(ctx, path, repos)
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatalf("Analysis failed: %v", err)
	}
	log.Printf("Analysis complete for %d inputs [%v elapsed]", numRepos, time.Since(start))
}

func writeReposDB(ctx context.Context, g *graph.Graph, repos []*deps.Repo) error {
	for _, repo := range repos {
		if err := g.AddAll(ctx, repo); err != nil {
			return err
		}
	}
	return nil
}

func writeReposJSON(ctx context.Context, path string, repos []*deps.Repo) error {
	var enc jsonpb.Marshaler
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, repo := range repos {
		if i > 0 {
			buf.WriteByte(',')
		}
		if err := enc.Marshal(&buf, repo); err != nil {
			return err
		}
	}
	buf.WriteString("]\n")
	out.Lock()
	defer out.Unlock()
	_, err := io.Copy(out, &buf)
	return err
}
