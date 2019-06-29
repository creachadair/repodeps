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

// Program depdomains scans a graph database and generates a histogram of
// dependencies by import path domain.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var limit = flag.Int("limit", 0, "Show only this many top order statistics")

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph()
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var numPkgs, numDeps int64
	dhist := make(map[string]int64)
	phist := make(map[string]int64)
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		numPkgs++
		numDeps += int64(len(row.Directs))
		seen := stringset.New()
		for _, ip := range row.Directs {
			prefix := strings.SplitN(ip, "/", 2)[0]
			isDom := strings.Index(prefix, ".") > 0
			if !isDom {
				continue // skip non-domain imports
			}

			// Record that this package depends on something in the prefix.
			if !seen.Contains(prefix) {
				seen.Add(prefix)
				phist[prefix]++
			}

			// Count how many things in the prefix this package depends on.
			dhist[prefix]++
		}
		return nil
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	// Output headers
	tw := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	fmt.Fprintf(tw, "PKGS\t%d\n", numPkgs) // total packages scanned
	fmt.Fprintf(tw, "DEPS\t%d\n", numDeps) // total dependencies observed
	fmt.Fprint(tw, "PATH\tDEPS\t%DEPS\tPKGS\t%PKGS\n")

	dkeys := stringset.FromKeys(dhist).Unordered()
	sort.Slice(dkeys, func(i, j int) bool {
		return dhist[dkeys[j]] < dhist[dkeys[i]]
	})
	if *limit > 0 && len(dkeys) > *limit {
		dkeys = dkeys[:*limit]
	}
	for _, key := range dkeys {
		fmt.Fprintf(tw, "%s\t%d\t%3.2g%%\t%d\t%3.2g%%\n", key,
			dhist[key], 100*float64(dhist[key])/float64(numDeps),
			phist[key], 100*float64(phist[key])/float64(numPkgs),
		)
	}
	tw.Flush()
}
