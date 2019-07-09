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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"sort"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
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

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Find the packages to analyze.
	paths := flag.Args()
	if *repoPath != "" {
		repos, err := local.Load(ctx, *repoPath, nil)
		if err != nil {
			log.Fatalf("Loading %q failed: %v", *repoPath, err)
		}
		for _, repo := range repos {
			for _, pkg := range repo.Packages {
				paths = append(paths, pkg.ImportPath)
			}
		}
	}
	pkgs := tools.NewMatcher(paths)

	// Compute reverse dependencies for the named packages.
	revDeps := make(map[string][]string)
	pkgRepo := make(map[string]string)
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		for _, pkg := range row.Directs {
			if pkgs(pkg) {
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
		grp, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(8)
		for _, url := range stringset.FromValues(pkgRepo).Elements() {
			url := tools.FixRepoURL(url)
			run(func() error {
				cmd := exec.CommandContext(ctx, "git", "-C", *clonePath, "clone", url)
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("fetching %q: %v", url, err)
				}
				log.Printf("Cloned %q", url)
				return nil
			})
		}
		if err := grp.Wait(); err != nil {
			log.Fatalf("Cloning failed: %v", err)
		}
	}

	// Write output.
	enc := json.NewEncoder(os.Stdout)
	for pkg, deps := range revDeps {
		rmap := make(map[string][]string)
		for _, dep := range deps {
			rmap[pkgRepo[dep]] = append(rmap[pkgRepo[dep]], dep)
		}
		out := output{Pkg: pkg}
		for repo, deps := range rmap {
			sort.Strings(deps)
			out.Deps = append(out.Deps, oneRepo{
				Repo: repo,
				Pkgs: deps,
			})
		}
		enc.Encode(out)
	}
}

type output struct {
	Pkg  string    `json:"package"`
	Deps []oneRepo `json:"affected"`
}

type oneRepo struct {
	Repo string   `json:"repository"`
	Pkgs []string `json:"packages"`
}