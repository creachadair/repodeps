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
	"regexp"
	"strings"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/storage"
)

// Reverse enumerates the reverse dependencies of one or more packages.  The
// order of results is unspecified but deterministic.
func (u *Server) Reverse(ctx context.Context, req *ReverseReq) (*ReverseRsp, error) {
	match, filter, err := req.compile(ctx, u.graph)
	if err != nil {
		return nil, err
	}
	repo := newRepoMap(ctx, u.graph)

	start := string(req.PageKey)
	rsp := new(ReverseRsp)
	err = u.graph.Scan(ctx, start, func(row *graph.Row) error {
		// If this row's package does not match the required regexp, skip it.
		if !filter(row.ImportPath) {
			return nil
		}
		// Look for imports of candidate packages in each row.
		var hits []string
		for _, dep := range row.Directs {
			if !match(dep) {
				continue // not one of the packages we care about
			} else if req.FilterSameRepo && repo.same(row.ImportPath, dep) {
				continue // package is in the same repository
			}
			hits = append(hits, dep)
		}
		rsp.NumImports += len(hits)

		// If reading this row will blow the limit, skip it till the next pass.
		// To ensure we make progress even if one package has a huge number of
		// imports, however, blow the limit anyway if this is the first matching
		// row we found.
		if len(rsp.Imports) != 0 && len(rsp.Imports)+len(hits) > req.Limit {
			rsp.NumImports -= len(hits)
			rsp.NextPage = []byte(row.ImportPath)
			return storage.ErrStopScan
		} else if req.CountOnly {
			return nil
		}

		for _, hit := range hits {
			rsp.Imports = append(rsp.Imports, &ReverseDep{
				Target: hit,
				Source: row.ImportPath,
			})
		}
		return nil
	})
	return rsp, err
}

// ReverseReq is the request parameter to the Reverse method.
type ReverseReq struct {
	// Find reverse dependencies for this package. If package ends with "/...",
	// any row with that prefix is matched.
	Package string `json:"package"`

	// Only count the number of matching rows; do not emit them.
	CountOnly bool `json:"countOnly"`

	// Filter out dependencies from packages in the same repository as the
	// target package or packages.
	FilterSameRepo bool `json:"filterSameRepo"`

	// If set, select only reverse dependencies matching this regexp.
	Matching string `json:"matching"`

	// Return at most this many rows (0 uses a reasonable default).
	Limit int `json:"limit"`

	// Resume reading from this page key.
	PageKey []byte `json:"pageKey"`
}

// compile returns a mapping from the candidate packages to their enclosing
// repositories.
func (m *ReverseReq) compile(ctx context.Context, db *graph.Graph) (match, filter func(string) bool, err error) {
	if m.Limit <= 0 {
		m.Limit = 50
	}

	// Compile the dependency filter.
	if m.Matching != "" {
		exp := strings.TrimPrefix(m.Matching, "(?!)")
		ok := exp == m.Matching
		r, err := regexp.Compile(exp)
		if err != nil {
			return nil, nil, err
		}
		filter = func(pkg string) bool { return r.MatchString(pkg) == ok }
	} else {
		filter = func(string) bool { return true }
	}

	// Compile the matching filter.
	if t := strings.TrimSuffix(m.Package, "/..."); t != m.Package && t != "" {
		match = func(pkg string) bool { return strings.HasPrefix(pkg, t) }
	} else {
		match = func(pkg string) bool { return pkg == m.Package }
	}
	return
}

// A ReverseDep encodes a single reverse direct dependency relationship. The
// source package directly imports the target package.
type ReverseDep struct {
	Target string `json:"target"` // the target (imported) package
	Source string `json:"source"` // the source (importing) package
}

// ReverseRsp is the response from a successful Reverse query.  If additional
// results are available, NextPage will contain an opaque page key that can be
// passed to retrieve the next chunk of results.
type ReverseRsp struct {
	NumImports int           `json:"numImports"`
	Imports    []*ReverseDep `json:"imports,omitempty"`
	NextPage   []byte        `json:"nextPage,omitempty"`
}

type repoMap struct {
	ctx context.Context
	g   *graph.Graph
	m   map[string]string
}

func newRepoMap(ctx context.Context, g *graph.Graph) *repoMap {
	return &repoMap{ctx: ctx, g: g, m: make(map[string]string)}
}

func (r *repoMap) same(a, b string) bool {
	ra, ok := r.m[a]
	if !ok {
		row, err := r.g.Row(r.ctx, a)
		if err != nil {
			return false
		}
		ra = row.Repository
		r.m[a] = ra
	}
	rb, ok := r.m[b]
	if !ok {
		row, err := r.g.Row(r.ctx, b)
		if err != nil {
			return false
		}
		rb = row.Repository
		r.m[b] = rb
	}
	return ra == rb
}
