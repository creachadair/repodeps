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

// Program listdeps lists the keys and values of a graph.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
	"github.com/golang/protobuf/jsonpb"
)

var (
	storePath   = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	doKeysOnly  = flag.Bool("keys", false, "Print only keys, not values")
	doFilterDom = flag.Bool("domain-only", false, "Print only import paths that begin with a domain")
	matchRepo   = flag.String("repo", "", "List only rows matching this repository")
)

func main() {
	flag.Parse()

	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	pfxs := flag.Args()
	if len(pfxs) == 0 {
		pfxs = append(pfxs, "") // list all
	}
	ctx := context.Background()
	var enc jsonpb.Marshaler
	if *matchRepo != "" {
		*matchRepo = tools.FixRepoURL(*matchRepo)
	}

	for _, pfx := range pfxs {
		err := g.Scan(ctx, pfx, func(row *graph.Row) error {
			if *matchRepo != "" && row.Repository != *matchRepo {
				return nil // skip non-matching repositories
			} else if _, ok := tools.HasDomain(row.ImportPath); !ok && *doFilterDom {
				return nil // skip non-domain paths
			} else if *doKeysOnly {
				fmt.Println(row.ImportPath)
				return nil
			}

			defer fmt.Println()
			return enc.Marshal(os.Stdout, row)
		})
		if err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
	}
}
