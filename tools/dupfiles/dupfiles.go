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

// Program dupfiles finds package source files with duplicate content.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	type loc struct {
		Repo       string `json:"repo"`
		Name       string `json:"path"`
		ImportPath string `json:"importPath"`
	}
	dups := make(map[string][]loc)

	ctx := context.Background()
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		for _, file := range row.SourceFiles {
			digest := string(file.Digest)
			elt := dups[digest]
			v := loc{
				Repo:       row.Repository,
				Name:       filepath.Base(file.RepoPath),
				ImportPath: row.ImportPath,
			}
			if elt == nil {
				dups[digest] = []loc{v}
			} else {
				dups[digest] = append(elt, v)
			}
		}
		return nil
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	tw := tabwriter.NewWriter(os.Stdout, 4, 8, 1, ' ', 0)
	for digest, locs := range dups {
		if len(locs) < 2 {
			continue
		}
		fmt.Fprintf(tw, "%x\n", digest)
		for _, elt := range locs {
			fmt.Fprintf(tw, "\t%s\t%s\t%s\n", elt.Name, elt.Repo, elt.ImportPath)
		}
		tw.Flush()
	}
}
