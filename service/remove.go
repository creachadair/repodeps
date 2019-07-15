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
	"encoding/json"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
)

// Remove removes package and repositories from the database.
func (u *Server) Remove(ctx context.Context, req *RemoveReq) (*RemoveRsp, error) {
	pkgs := stringset.New()
	for _, pkg := range req.Package {
		if err := u.graph.Remove(ctx, pkg); err == storage.ErrKeyNotFound {
			continue
		} else if err != nil {
			u.pushLog(ctx, req.LogErrors, "log.removePackage", struct {
				P string `json:"package"`
				M string `json:"message"`
			}{P: pkg, M: err.Error()})
		} else {
			pkgs.Add(pkg)
		}
	}
	repos := stringset.FromIndexed(len(req.Repository), func(i int) string {
		return poll.FixRepoURL(req.Repository[i])
	})
	if len(repos) != 0 {
		for repo := range repos {
			if err := u.repoDB.Remove(ctx, repo); err != nil {
				u.pushLog(ctx, req.LogErrors, "log.removeRepo", err)
			}
		}
		if err := u.graph.Scan(ctx, "", func(row *graph.Row) error {
			if repos.Contains(row.Repository) {
				err := u.graph.Remove(ctx, row.ImportPath)
				if err != nil {
					u.pushLog(ctx, req.LogErrors, "log.removePackage", err)
				} else {
					pkgs.Add(row.ImportPath)
				}
				return nil
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	return &RemoveRsp{
		Repositories: repos.Elements(),
		Packages:     pkgs.Elements(),
	}, nil
}

// RemoveReq is the request parameter to the Remove method.
type RemoveReq struct {
	Repository StringList `json:"repository"`
	Package    StringList `json:"package"`
	LogErrors  bool       `json:"logErrors"`
}

// RemoveRsp is the result from a successful Remove call.
type RemoveRsp struct {
	Repositories []string `json:"repositories,omitempty"` // repositories removed
	Packages     []string `json:"packages,omitempty"`     // packages removed
}

// A StringList is a slice of strings that can be decoded from JSON as either
// an array or a single string.
type StringList []string

// UnmarshalJSON decodes a StringList from JSON, accepting either a string
// value (corresponding to a single-element slice) or an array of strings.
func (s *StringList) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*s = nil
		return nil
	} else if data[0] == '"' {
		*s = []string{""}
		return json.Unmarshal(data, &(*s)[0])
	}
	return json.Unmarshal(data, (*[]string)(s))
}
