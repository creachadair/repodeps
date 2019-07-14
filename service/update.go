package service

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/poll"
)

// Update processes a single update request. An error has concrete type
// *jrpc2.Error and errors during the update phase have a partial response
// attached as a data value.
func (u *Server) Update(ctx context.Context, req *UpdateReq) (*UpdateRsp, error) {
	if u.opts.ReadOnly {
		return nil, errors.New("database is read-only")
	} else if req.Repository == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing repository URL")
	} else if req.CheckOnly && req.Force {
		return nil, jrpc2.Errorf(code.InvalidParams, "checkOnly and force are mutually exclusive")
	}
	repoTag := poll.FixRepoURL(req.Repository)
	if req.Reference != "" {
		repoTag += "@@" + req.Reference
	}
	res, err := u.repoDB.Check(ctx, repoTag)
	if err != nil {
		return nil, jrpc2.Errorf(code.SystemError, "checking %s: %v", req.Repository, err)
	}

	out := &UpdateRsp{
		Repository:  res.URL,
		NeedsUpdate: res.NeedsUpdate(),
		Reference:   res.Name,
		Digest:      res.Digest,
		Errors:      res.Errors,
	}
	if u.opts.ErrorLimit > 0 && out.Errors >= u.opts.ErrorLimit {
		u.repoDB.Remove(ctx, out.Repository)
		out.Removed = true
		return nil, jrpc2.DataErrorf(code.SystemError, out, "removed after %d failures", out.Errors)
	} else if req.CheckOnly {
		return out, nil
	}

	if out.NeedsUpdate || req.Force {
		// If the caller requested a reset, remove all packages matching this
		// repository before performing the update.
		if req.Reset {
			u.graph.Scan(ctx, "", func(row *graph.Row) error {
				if row.Repository != res.URL {
					return nil
				} else if err := u.graph.Remove(ctx, row.ImportPath); err != nil {
					log.Printf("[remove failed] %q: %v", row.ImportPath, err)
					// TODO: Push back a log notification?
				}
				return nil
			})
		}

		// Clone the repository at the target head and update its packages.
		np, err := u.cloneAndUpdate(ctx, res, u.opts.merge(req.Options))
		out.NumPackages = np
		if err != nil {
			return nil, jrpc2.DataErrorf(code.SystemError, out, "update %s: %v", res.URL, err)
		}
	}
	return out, nil
}

// UpdateReq is the request parameter to the Update method.
type UpdateReq struct {
	// The URL of the repository to update, must be non-empty.
	Repository string `json:"repository"`

	// The reference name to update (optional).
	Reference string `json:"reference"`

	// If true, only check the repository state, do not update.
	CheckOnly bool `json:"checkOnly"`

	// If true, remove any packages currently attributed to this repository
	// before updating.
	Reset bool `json:"reset"`

	// If true, force an update even if one is not needed.
	Force bool `json:"force"`

	// Options for the package loader (if unset, service defaults are used).
	Options *deps.Options `json:"options"`
}

// UpdateRsp is the response from a successful Update call.
type UpdateRsp struct {
	Repository  string `json:"repository"`  // the fetch URL of the repository
	NeedsUpdate bool   `json:"needsUpdate"` // whether an update was needed
	Reference   string `json:"reference"`   // the name of the target reference
	Digest      string `json:"digest"`      // the SHA-1 digest (hex) at the reference

	NumPackages int  `json:"numPackages,omitempty"` // the number of packages updated
	Errors      int  `json:"errors,omitempty"`      // number of consecutive update failures
	Removed     bool `json:"removed,omitempty"`     // true if removed due to the error limit
}

func (u *Server) cloneAndUpdate(ctx context.Context, res *poll.CheckResult, opts *deps.Options) (int, error) {
	path, err := ioutil.TempDir(u.opts.WorkDir, res.Digest)
	if err != nil {
		return 0, fmt.Errorf("creating clone directory: %v", err)
	}
	defer os.RemoveAll(path) // best-effort cleanup
	if err := res.Clone(ctx, path); err != nil {
		return 0, fmt.Errorf("cloning %v", err)
	}
	repos, err := local.Load(ctx, path, opts)
	if err != nil {
		return 0, fmt.Errorf("loading: %v", err)
	}
	var added int
	for _, repo := range repos {
		if err := u.graph.AddAll(ctx, repo); err != nil {
			return added, err
		}
		added += len(repo.Packages)
	}
	return added, nil
}
