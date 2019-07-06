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

// Program rankdeps computes an iterative PageRank value for a graph.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var (
	storePath   = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	numIter     = flag.Int("iterations", 10, "Number of iterations")
	dampFactor  = flag.Float64("damping", 0.85, "Damping factor (0..1)")
	scaleFactor = flag.Float64("scale", 100, "Scaling factor for rank values")
	doUpdate    = flag.Bool("update", false, "Write ranks back into the graph")
)

func main() {
	flag.Parse()
	mode := tools.ReadOnly
	if *doUpdate {
		mode = tools.ReadWrite
	}
	g, c, err := tools.OpenGraph(*storePath, mode)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	// Load and initialize the link graph.
	ctx := context.Background()
	m := make(linkMap)
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		m[row.ImportPath] = &node{cur: 1, links: row.Directs}
		return nil
	}); err != nil {
		log.Fatalf("Initializing link graph: %v", err)
	}
	if len(m) == 0 {
		return // nothing to do
	}
	log.Printf("Loaded graph with %d nodes", len(m))

	// Iterate the ranking algorithm.
	log.Printf("Computing %d ranking iterations...", *numIter)
	start := time.Now()
	m.initialize()
	for i := 1; i <= *numIter; i++ {
		fmt.Fprint(os.Stderr, ".")
		m.push()
		m.advance()
	}
	fmt.Fprintln(os.Stderr)
	log.Printf("Rank computation complete [%v elapsed]", time.Since(start))

	// Find the maximum ranking to use as the apex of the scale, then scale all
	// the rankings by that value.
	//
	// Precondition: cur is the ranking value for each node.
	// Postcondition: next is the scaled ranking value for each node.
	max := 0.0
	for _, elt := range m {
		if elt.cur > max {
			max = elt.cur
		}
	}
	sf := float64(*scaleFactor) / max
	for _, elt := range m {
		elt.next = elt.cur * sf
	}

	if *doUpdate {
		log.Print("Updating database...")
		start := time.Now()
		var numRows, numRanks int64
		if err := g.ScanUpdate(ctx, "", func(row *graph.Row) bool {
			numRows++
			if elt, ok := m[row.ImportPath]; ok {
				row.Ranking = elt.next
				numRanks++
				return true
			}
			return false
		}); err != nil {
			log.Fatalf("Update failed: %v", err)
		}
		log.Printf("Updated %d ranks in %d rows [%v elapsed]", numRanks, numRows, time.Since(start))
		return
	}

	// Otherwise, write the results to stdout.
	for key, node := range m {
		fmt.Printf("%s\t%.2f\n", key, node.next)
	}
}

type node struct {
	cur, next float64
	links     []string
}

type linkMap map[string]*node

// initialize adds any links that do not have a row in the graph
// to the link map, and normalizes each node's weight by the total
// number of nodes.
func (m linkMap) initialize() {
	for _, elt := range m {
		for _, link := range elt.links {
			if _, ok := m[link]; !ok {
				m[link] = &node{cur: 1} // no outbound links here
			}
		}
	}
	n := float64(len(m))
	for _, elt := range m {
		elt.cur /= n
	}
}

// advance advances each node to its next value and applies the damping factor.
// Precondition: next is the undamped sum of propagated weights from in-edges.
// Postcondition: next == 0 at each node.
func (m linkMap) advance() {
	adj := 1 - *dampFactor
	for _, elt := range m {
		elt.cur = adj + *dampFactor*elt.next
		elt.next = 0
	}
}

// push pushes the current weight of each node to each of its targets.
// Precondition: next == 0 for each node.
func (m linkMap) push() {
	for _, elt := range m {
		if len(elt.links) == 0 {
			continue // no links to push to

			// In the random explorer model, this node should give its
			// weight to all other nodes. Here we just assume that effect
			// is negligible on a graph of any interesting size.
		}
		w := elt.cur / float64(len(elt.links))
		for _, link := range elt.links {
			m[link].next += w
		}
	}
}
