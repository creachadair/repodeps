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

// Program removedeps removes rows from the graph.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var (
	storePath  = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	rmPackages = flag.Bool("pkg", false, "Remove the rows for the named packages")
	rmRepo     = flag.String("repo", "", "Remove all rows claimed by this repository")
)

func main() {
	flag.Parse()
	if *rmPackages == (*rmRepo != "") {
		log.Fatal("You must set exactly one of -pkg or -repo")
	}
	g, c, err := tools.OpenGraph(*storePath, tools.ReadWrite)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()
	ctx := context.Background()
	start := time.Now()

	var numRemoved int
	if *rmRepo != "" {
		needle := tools.CleanRepoURL(*rmRepo)
		log.Printf("Removing packages from %q...", needle)

		if err := g.Scan(ctx, "", func(row *graph.Row) error {
			if row.Repository == needle {
				if err := g.Remove(ctx, row.ImportPath); err == nil {
					numRemoved++
					log.Printf("[removed] %q", row.ImportPath)
				} else {
					log.Printf("[skipped] %q: %v", row.ImportPath, err)
				}
			}
			return nil
		}); err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
	} else {
		for _, pkg := range flag.Args() {
			if err := g.Remove(ctx, pkg); err == nil {
				numRemoved++
				log.Printf("[removed] %q", pkg)
			} else {
				log.Printf("[skipped] %q: %v", pkg, err)
			}
		}
	}
	log.Printf("Removed %d packages [%v elapsed]", numRemoved, time.Since(start))
}
