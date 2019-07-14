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

// Program resolverepo attempts to resolve an import path using a vanity domain
// to the underlying repository address.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

var (
	doReadStdin  = flag.Bool("stdin", false, "Read import paths from stdin")
	doKeepErrors = flag.Bool("keep-errors", false, "Keep repositories with errors")
	doDropEmpty  = flag.Bool("drop-empty", false, "Discard repositories with no import paths")
	concurrency  = flag.Int("concurrency", 16, "Number of concurrent workers")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <import-path>...

Resolve Go import paths to Git repository URLs for vanity domains.  The
resolution algorithm is borrowed from the "go get" command, which issues an
HTTP query to the hosting site to request import information.

For each resolved repository, the tool prints a JSON text to stdout having the
fields:

  {
    "repository":  "repository fetch URL (string)",
    "prefix":      "import path prefix covered by this repository (string)",
    "importPaths": ["import paths (array of strings)"]
  }

The non-flag arguments name the import paths to resolve. With -stdin, each line
of stdin will be read as an additional import path to resolve.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	// Accumulated repository mappings, becomes output.
	repos := newRepoMap()

	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan *metaImport, *concurrency)
	start := time.Now()

	// Collect lookup results and update the repository map.
	go func() {
		defer cancel()
		for imp := range results {
			repos.set(imp)
		}
	}()

	// Process inputs and send results to the collector.
	g, run := taskgroup.New(nil).Limit(*concurrency)
	for ip := range tools.Inputs(*doReadStdin) {
		run(func() error {
			if repos.find(ip) {
				return nil // already handled
			}
			results <- resolveImportRepo(ctx, ip)
			return nil
		})
	}

	// Wait for all the workers to complete, then signal the collector.
	err := g.Wait()
	close(results)
	if err != nil {
		log.Fatalf("Processing failed: %v", err)
	}

	<-ctx.Done() // wait for the collector to complete

	log.Printf("[done] %d repositories found [%v elapsed]", repos.len(), time.Since(start))

	// Encode the output.
	enc := json.NewEncoder(os.Stdout)
	for _, b := range repos.m {
		if b.Error != "" && !*doKeepErrors || len(b.ImportPaths) == 0 && *doDropEmpty {
			continue
		}
		sort.Strings(b.ImportPaths)
		if err := enc.Encode(b); err != nil {
			log.Fatalf("Encoding failed: %v", err)
		}
	}
}

type bundle struct {
	Repo        string   `json:"repository"`
	Prefix      string   `json:"prefix"`
	ImportPaths []string `json:"importPaths,omitempty"`
	Error       string   `json:"error,omitempty"`
}

// resolveImportRepo attempts to resolve the URL of the specified import path
// using the HTTP metadata protocol used by "go get". Unlike "go get", this
// resolver only considers Git targets.
func resolveImportRepo(ctx context.Context, ipath string) *metaImport {
	rsp, err := service.ResolveRepository(ctx, &service.ResolveReq{
		Package: ipath,
	})
	return &metaImport{Repo: rsp, Err: err}
}

type metaImport struct {
	Repo *service.ResolveRsp
	Err  error
}

type repoMap struct {
	μ sync.RWMutex
	m map[string]*bundle
}

func newRepoMap() *repoMap {
	return &repoMap{m: make(map[string]*bundle)}
}

func (r *repoMap) len() int {
	r.μ.RLock()
	defer r.μ.RUnlock()
	return len(r.m)
}

func (r *repoMap) find(ip string) bool {
	r.μ.RLock()
	defer r.μ.RUnlock()
	for pfx, b := range r.m {
		if strings.HasPrefix(ip, pfx) {
			b.ImportPaths = append(b.ImportPaths, ip)
			return true
		}
	}
	return false
}

func (r *repoMap) set(imp *metaImport) {
	r.μ.Lock()
	defer r.μ.Unlock()
	b := r.m[imp.Repo.Prefix]
	if b == nil {
		b = &bundle{
			Repo:   imp.Repo.Repository,
			Prefix: imp.Repo.Prefix,
		}
		if imp.Err != nil {
			b.Error = imp.Err.Error()
		}
		r.m[imp.Repo.Prefix] = b
	} else if b.Error == "" && imp.Repo.ImportPath != "" {
		b.ImportPaths = append(b.ImportPaths, imp.Repo.ImportPath)
	}
}
