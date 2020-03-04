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
	"log"
	"os"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/service"
)

var (
	address = flag.String("address", os.Getenv("DEPSERVER_ADDR"), "Service address")

	repoPath   = flag.String("repo", "", "Path to local repository to analyze")
	filterSame = flag.Bool("filter-same-repo", false, "Exclude dependencies from the same repository")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	// Find the packages to analyze.
	paths := flag.Args()
	if *repoPath != "" {
		repos, err := local.Load(ctx, *repoPath, &deps.Options{
			UseImportComments: true,
		})
		if err != nil {
			log.Fatalf("Loading %q failed: %v", *repoPath, err)
		}
		for _, repo := range repos {
			for _, pkg := range repo.Packages {
				paths = append(paths, pkg.ImportPath)
			}
		}
	}
	if len(paths) == 0 {
		log.Fatal("No packages to analyze")
	}

	// Compute reverse dependencies for the named packages.
	allPkgs := stringset.New()
	byRepo := make(map[string]stringset.Set) // affected packages by repo
	if _, err := c.Reverse(ctx, &service.ReverseReq{
		Package:        paths,
		FilterSameRepo: *filterSame,
		Complete:       true,
	}, func(dep *service.ReverseDep) error {
		allPkgs.Add(dep.Target)
		s, ok := byRepo[dep.Row.Repository]
		if !ok {
			s = stringset.New()
			byRepo[dep.Row.Repository] = s
		}
		s.Add(dep.Row.ImportPath)
		return nil
	}); err != nil {
		log.Fatalf("Reverse lookup failed: %v", err)
	}

	// Write output.
	out := output{Pkgs: allPkgs.Elements()}
	for repo, deps := range byRepo {
		out.Deps = append(out.Deps, oneRepo{
			Repo: repo,
			Pkgs: deps.Elements(),
		})
	}
	json.NewEncoder(os.Stdout).Encode(out)
}

type output struct {
	Pkgs []string  `json:"packages"`
	Deps []oneRepo `json:"affected"`
}

type oneRepo struct {
	Repo string   `json:"repository"`
	Pkgs []string `json:"packages"`
}
