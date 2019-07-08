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

// Program depdistance computes minimum dependency distance vectors.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"time"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var (
	storePath   = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	doFilterDom = flag.Bool("domain-only", false, "Print only import paths that begin with a domain")
)

func main() {
	flag.Parse()

	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	const inf = math.MaxInt32
	ids := make(map[string]int32)          // :: import path → id
	rid := make(map[int32]string)          // :: id → import path
	dmx := make(map[int32]map[int32]int32) // :: id → id → dist
	A := func(pkg string) int32 {
		v, ok := ids[pkg]
		if !ok {
			v = int32(len(ids)) + 1
			ids[pkg] = v
			rid[v] = pkg
		}
		return v
	}
	get := func(a, b int32) int32 {
		if v, ok := dmx[a][b]; ok {
			return v
		}
		return inf
	}
	set := func(a, b, d int32) {
		m := dmx[a]
		if m == nil {
			m = make(map[int32]int32)
			dmx[a] = m
		}
		m[b] = d
	}

	// Set up a base matrix with single-step distances.
	start := time.Now()
	ctx := context.Background()
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		if _, ok := deps.HasDomain(row.ImportPath); !ok && *doFilterDom {
			return nil
		}
		src := A(row.ImportPath)
		set(src, src, 0)
		for _, pkg := range row.Directs {
			set(A(pkg), src, 1)
		}
		return nil
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}
	log.Printf("Loaded %d nodes [%v elapsed]", len(ids), time.Since(start))

	// Floyd-Warshall.
	start = time.Now()
	for i := int32(0); i < int32(len(ids)); i++ {
		for j := int32(0); j < int32(len(ids)); j++ {
			for k := int32(0); k < int32(len(ids)); k++ {
				dij, dik, dkj := get(i, j), get(i, k), get(k, j)
				if dik == inf || dkj == inf {
					continue // no path
				} else if v := dik + dkj; dij > v {
					set(i, j, v)
				}
			}
			if j%1000 == 0 {
				fmt.Fprint(os.Stderr, ".")
			}
		}
		if i%1000 == 0 {
			fmt.Fprintln(os.Stderr, "+")
		}
	}
	fmt.Fprintln(os.Stderr)
	log.Printf("Computed all pairs shortest paths [%v elapsed]", time.Since(start))

	// Output: from <tab> to <tab> distance <eol>
	for i := int32(0); i < int32(len(ids)); i++ {
		for j := int32(0); j < int32(len(ids)); j++ {
			if v := get(i, j); v != inf && v > 0 {
				fmt.Printf("%s\t%s\t%d\n", rid[i], rid[j], v)
			}
		}
	}
}
