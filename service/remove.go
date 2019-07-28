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
	"fmt"

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
	bases := stringset.New()
	for repo := range repos {
		tags, err := u.repoDB.Tags(ctx, repo)
		if err != nil {
			u.pushLog(ctx, req.LogErrors, "log.removeRepo", fmt.Errorf("repo %s: %v", repo, err))
			continue
		}
		for _, stat := range tags {
			if stat.Key != "" {
				err = u.repoDB.Remove(ctx, stat.Key)
			} else {
				err = u.repoDB.Remove(ctx, repo)
			}
			if err == nil {
				bases.Add(stat.Repository)
			} else {
				u.pushLog(ctx, req.LogErrors, "log.removeRepo", fmt.Errorf("tag %s: %v", stat.Key, err))
			}
		}
	}
	if !req.KeepPackages && len(bases) != 0 {
		err := u.graph.Scan(ctx, "", func(row *graph.Row) error {
			if bases.Contains(row.Repository) {
				if err := u.graph.Remove(ctx, row.ImportPath); err != nil {
					u.pushLog(ctx, req.LogErrors, "log.removePackage", fmt.Errorf("pkg %s: %v", row.ImportPath, err))
				} else {
					pkgs.Add(row.ImportPath)
				}
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return &RemoveRsp{
		Repositories: bases.Elements(),
		Packages:     pkgs.Elements(),
	}, nil
}

// RemoveReq is the request parameter to the Remove method.
type RemoveReq struct {
	Repository   StringList `json:"repository"`
	Package      StringList `json:"package"`
	KeepPackages bool       `json:"keepPackages"`
	LogErrors    bool       `json:"logErrors"`
}

// RemoveRsp is the result from a successful Remove call.
type RemoveRsp struct {
	Repositories []string `json:"repositories,omitempty"` // repositories removed
	Packages     []string `json:"packages,omitempty"`     // packages removed
}
