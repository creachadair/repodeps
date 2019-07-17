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
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
)

// Match enumerates the rows of the graph matching the specified query.  If
// more rows are available than the limit requested, the response will indicate
// the next offset of a matching row.
func (u *Server) Match(ctx context.Context, req *MatchReq) (*MatchRsp, error) {
	matchPackage, matchRepo, start := req.compile()
	if req.Limit <= 0 {
		req.Limit = u.opts.DefaultPageSize
	}

	rsp := new(MatchRsp)
	err := u.graph.Scan(ctx, start, func(row *graph.Row) error {
		if !matchRepo(row.Repository) {
			return nil // row does not match
		} else if !matchPackage(row.ImportPath) {
			return storage.ErrStopScan // no more matches are possible
		}

		if req.CountOnly {
			// do nothing
		} else if len(rsp.Rows) < req.Limit {
			rsp.Rows = append(rsp.Rows, row)
			if !req.IncludeSource {
				row.SourceFiles = nil
			}
			if req.ExcludeDirects {
				row.Directs = nil
			}
		} else {
			// Found the starting point for the next page.
			rsp.NextPage = []byte(row.ImportPath)
			return storage.ErrStopScan
		}
		rsp.NumRows++
		return nil
	})
	return rsp, err
}

// MatchReq is the request parameter to the Match method.
type MatchReq struct {
	// Match rows for this package. If package ends with "/...", any row with
	// that prefix is matched.
	Package string `json:"package"`

	// Match rows with this repository URL.
	Repository string `json:"repository"`

	// Only count the number of matching rows; do not emit them.
	CountOnly bool `json:"countOnly"`

	// Whether to include source file paths.
	IncludeSource bool `json:"includeSource"`

	// Whether to exclude direct dependencies.
	ExcludeDirects bool `json:"excludeDirects"`

	// Return at most this many rows (0 uses a reasonable default).
	Limit int `json:"limit"`

	// Resume reading from this page key.
	PageKey []byte `json:"pageKey"`
}

func (m *MatchReq) compile() (mpkg, mrepo func(string) bool, start string) {
	mpkg = func(string) bool { return true }
	if t := strings.TrimSuffix(m.Package, "/..."); t != m.Package && t != "" {
		start = t
		mpkg = func(pkg string) bool { return strings.HasPrefix(pkg, t) }
	} else if m.Package != "" {
		start = m.Package
		mpkg = func(pkg string) bool { return pkg == m.Package }
	}

	mrepo = func(string) bool { return true }
	if m.Repository != "" {
		fixed := poll.FixRepoURL(m.Repository)
		mrepo = func(repo string) bool { return repo == fixed }
	}

	if s := string(m.PageKey); s != "" {
		start = s
	}
	return
}

// MatchRsp is the response from a successful Match query.
type MatchRsp struct {
	// The number of rows processed to obtain this result. If countOnly was true
	// in the request, this is the total number of matching rows.
	NumRows int `json:"numRows"`

	Rows     []*graph.Row `json:"rows,omitempty"`
	NextPage []byte       `json:"nextPage,omitempty"`
}
