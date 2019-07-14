package service

import (
	"context"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
)

// Remove removes package and repositories from the database.
func (u *Server) Remove(ctx context.Context, req *RemoveReq) (*RemoveRsp, error) {
	pkgs := stringset.New()
	for _, pkg := range req.Packages {
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
	repos := stringset.FromIndexed(len(req.Repositories), func(i int) string {
		return poll.FixRepoURL(req.Repositories[i])
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
	Repositories []string `json:"repositories"`
	Packages     []string `json:"packages"`
	LogErrors    bool     `json:"logErrors"`
}

// RemoveRsp is the result from a successful Remove call.
type RemoveRsp struct {
	Repositories []string `json:"repositories,omitempty"` // repositories removed
	Packages     []string `json:"packages,omitempty"`     // packages removed
}
