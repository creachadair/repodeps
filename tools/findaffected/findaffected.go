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

// Program findaffected locates packages in public repositories that need to be
// updated for breaking changes in a set of packages listed on the command line
// or inferred from the current working directory.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/tools"
)

var (
	storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	repoPath  = flag.String("repo", "", "Path to local repository to analyze")
	clonePath = flag.String("clone", "", "Clone repositories in this directory")
)

func main() {
	flag.Parse()

	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()

	// Find the packages to analyze.
	pkgs := stringset.New(flag.Args()...)
	if *repoPath != "" {
		repos, err := local.Load(ctx, *repoPath, nil)
		if err != nil {
			log.Fatalf("Loading %q failed: %v", *repoPath, err)
		}
		for _, repo := range repos {
			for _, pkg := range repo.Packages {
				pkgs.Add(pkg.ImportPath)
			}
		}
	}

	// Compute reverse dependencies for the named packages.
	revDeps := make(map[string][]string)
	pkgRepo := make(map[string]string)
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		for _, pkg := range row.Directs {
			if pkgs.Contains(pkg) {
				revDeps[pkg] = append(revDeps[pkg], row.ImportPath)
				pkgRepo[row.ImportPath] = row.Repository
			}
		}
		return nil
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// If requested, clone the repositories.
	if *clonePath != "" {
		if err := os.MkdirAll(*clonePath, 0700); err != nil {
			log.Fatalf("Creating fork directory: %v", err)
		}
		for _, url := range stringset.FromValues(pkgRepo).Elements() {
			path := filepath.Join(*clonePath, filepath.Base(url))
			res := &poll.CheckResult{URL: url, Digest: "refs/heads/master"}
			if err := res.Clone(ctx, path); err != nil {
				log.Fatalf("Cloning %q failed: %v", url, err)
			}
			log.Printf("Cloned %q", url)
		}
	}

	// Write output.
	for pkg, deps := range revDeps {
		rmap := make(map[string][]string)
		for _, dep := range deps {
			rmap[pkgRepo[dep]] = append(rmap[pkgRepo[dep]], dep)
		}
		fmt.Println(pkg)
		for repo, deps := range rmap {
			fmt.Println("  " + repo)
			sort.Strings(deps)
			fmt.Print("   - ", strings.Join(deps, "\n   - "), "\n")
		}
	}
}
