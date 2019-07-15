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

package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/graph"
)

// Rank computes pagerank for the nodes of the graph.
func (u *Server) Rank(ctx context.Context, req *RankReq) (*RankRsp, error) {
	if u.opts.ReadOnly && req.Update {
		return nil, errors.New("database is read-only")
	}
	numIter := req.Iterations
	if numIter <= 0 {
		numIter = u.opts.RankIterations
	}
	damp := req.Damping
	if damp == 0 {
		damp = u.opts.RankDamping
	}
	scale := req.Scale
	if scale == 0 {
		scale = u.opts.RankScale
	}

	if numIter < 0 {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid iteration count %d", numIter)
	} else if damp < 0 || damp > 1 {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid damping factor %g", damp)
	} else if !u.tryScanning() {
		return nil, jrpc2.Errorf(code.SystemError, "scan already in progress")
	}
	defer u.doneScanning()

	rsp := new(RankRsp)
	start := time.Now()
	defer func() { rsp.Elapsed = time.Since(start) }()

	type progress struct {
		N int           `json:"n,omitempty"`
		R float64       `json:"rank,omitempty"`
		L string        `json:"msg"`
		E time.Duration `json:"elapsed,omitempty"`
	}

	// Load and populate the link graph.
	m := make(linkMap)
	if err := u.graph.Scan(ctx, "", func(row *graph.Row) error {
		m[row.ImportPath] = &node{cur: 1, links: row.Directs}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("initializing link graph: %v", err)
	}
	u.pushLog(ctx, req.LogProgress, "log.progress", progress{
		N: len(m), L: "graph loaded", E: time.Since(start),
	})
	rsp.NumRows = len(m)
	if len(m) == 0 {
		return rsp, nil // nothing to do
	}

	// Iterate the ranking algorithm.
	m.initialize()
	for i := 1; i <= numIter; i++ {
		m.push()
		m.advance(damp)
		u.pushLog(ctx, req.LogProgress, "log.progress", progress{
			N: i, L: "iteration complete", E: time.Since(start),
		})
	}
	u.pushLog(ctx, req.LogProgress, "log.progress", progress{
		L: "ranking complete", E: time.Since(start),
	})

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
	mul := math.Pow(10, float64(scale))
	for _, elt := range m {
		elt.next = math.Trunc(mul * (elt.cur / (max + 1)))
	}

	err := u.graph.ScanUpdate(ctx, "", func(row *graph.Row) bool {
		elt, ok := m[row.ImportPath]
		isDiff := ok && elt.next != row.Ranking
		isUpdate := req.Update && isDiff

		if isDiff {
			u.pushLog(ctx, req.LogUpdates, "log.updateRank", progress{R: elt.next, L: row.ImportPath})
			rsp.NumRanks++
		}
		if isUpdate {
			row.Ranking = elt.next
			return true
		}
		return false
	})
	return rsp, err
}

// RankReq is the request parameter to the Rank method.
type RankReq struct {
	// Number of iterations to compute; > 0 required.
	Iterations int `json:"iterations"`

	// Damping factor (0..1).
	Damping float64 `json:"damping"`

	// Scale values to this number of significant figures (0 means don't scale).
	Scale int `json:"scale"`

	// Write the updated rankings back to the database.
	Update bool `json:"update"`

	LogProgress bool `json:"logProgress"` // push progress notifications
	LogUpdates  bool `json:"logUpdates"`  // push update notifications
}

// RankRsp is the response from a successful Rank query.
type RankRsp struct {
	NumRows  int `json:"numRows"`  // total count of rows examined
	NumRanks int `json:"numRanks"` // number of rankings updated

	Elapsed time.Duration `json:"elapsed"`
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
func (m linkMap) advance(df float64) {
	adj := 1 - df
	for _, elt := range m {
		elt.cur = adj + df*elt.next
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
