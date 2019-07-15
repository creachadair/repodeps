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
	"strings"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/storage"
)

// Reverse enumerates the reverse dependencies of one or more packages.  The
// order of results is unspecified but deterministic.
func (u *Server) Reverse(ctx context.Context, req *ReverseReq) (*ReverseRsp, error) {
	pkgs, err := req.compile(ctx, u.graph)
	if err != nil {
		return nil, err
	}

	start := string(req.PageKey)
	rsp := new(ReverseRsp)
	err = u.graph.Scan(ctx, start, func(row *graph.Row) error {
		// Look for imports of candidate packages in each row.
		var hits []string
		for _, dep := range row.Directs {
			repo, ok := pkgs[dep]
			if ok && (!req.FilterSameRepo || row.Repository != repo) {
				hits = append(hits, dep)
			}
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

	// Return at most this many rows (0 uses a reasonable default).
	Limit int `json:"limit"`

	// Resume reading from this page key.
	PageKey []byte `json:"pageKey"`
}

// compile returns a mapping from the candidate packages to their enclosing
// repositories.
func (m *ReverseReq) compile(ctx context.Context, db *graph.Graph) (map[string]string, error) {
	start := m.Package
	match := func(pkg string) bool { return pkg == m.Package }
	if t := strings.TrimSuffix(m.Package, "/..."); t != m.Package && t != "" {
		start = t
		match = func(pkg string) bool { return strings.HasPrefix(pkg, t) }
	}
	pkgRepo := make(map[string]string)
	if err := db.Scan(ctx, start, func(row *graph.Row) error {
		if !match(row.ImportPath) {
			return storage.ErrStopScan
		}
		pkgRepo[row.ImportPath] = row.Repository
		return nil
	}); err != nil {
		return nil, err
	}
	if m.Limit <= 0 {
		m.Limit = 50
	}
	return pkgRepo, nil
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
