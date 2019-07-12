// Package service defines a service that maintains the state of a dependency
// graph. It is compatible with the github.com/creachadair/jrpc2 package, but
// can also be used independently.
package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/badgerstore"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/local"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

// Options control the behaviour of a Server.
type Options struct {
	RepoDB  string // path of repository state database (required)
	GraphDB string // path of graph database (required)
	WorkDir string // working directory for update clones

	// The minimum polling interval for repository scans.
	MinPollInterval time.Duration

	// The maximum number of times a repository update may fail before that
	// repository is removed from the database.
	ErrorLimit int

	// Default sampling rate for scans (0..1); zero means 1.0.
	SampleRate float64

	// The maximum number of concurrent workers that may be processing updates.
	// A value less that or equal to zero means 1.
	Concurrency int

	// If set, this callback is invoked to deliver streaming logs from scan
	// operations. The server ensures that at most one goroutine is active in
	// this callback at once.
	StreamLog func(ctx context.Context, key string, value interface{}) error

	// Default package loader options.
	deps.Options
}

// New constructs a new Server from the specified options.  As long as the
// server is open, it holds a lock on the databases assigned to it.
// The caller must call Close when the server is no longer in use.
func New(opts Options) (*Server, error) {
	if opts.RepoDB == "" {
		return nil, errors.New("no repository database")
	}
	if opts.GraphDB == "" {
		return nil, errors.New("no graph database")
	}
	if opts.SampleRate == 0 {
		opts.SampleRate = 1
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	u := &Server{opts: opts}
	if f := opts.StreamLog; f != nil {
		mu := new(sync.Mutex)
		u.log = func(ctx context.Context, key string, arg interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			return f(ctx, key, arg)
		}
	} else {
		u.log = jrpc2.ServerPush
	}

	if s, err := badgerstore.NewPath(opts.RepoDB); err == nil {
		u.repoDB = poll.NewDB(storage.NewBlob(s))
		u.repoC = s
	} else {
		return nil, fmt.Errorf("opening repository database: %v", err)
	}
	if s, err := badgerstore.NewPath(opts.GraphDB); err == nil {
		u.graph = graph.New(storage.NewBlob(s))
		u.graphC = s
	} else {
		u.repoC.Close()
		return nil, fmt.Errorf("opening graph database: %v", err)
	}

	return u, nil
}

// A Server manages reads and updates to a database of dependencies.
type Server struct {
	repoDB *poll.DB
	repoC  io.Closer
	graph  *graph.Graph
	graphC io.Closer

	scanning int32
	opts     Options

	log func(context.Context, string, interface{}) error
}

func (u *Server) tryScanning() bool {
	return atomic.AddInt32(&u.scanning, 1) == 1
}

func (u *Server) doneScanning() { atomic.StoreInt32(&u.scanning, 0) }

// Close shuts down the server and closes its underlying data stores.
func (u *Server) Close() error {
	gerr := u.graphC.Close()
	rerr := u.repoC.Close()
	if gerr != nil {
		return gerr
	}
	return rerr
}

// Match enumerates the rows of the graph matching the specified query.  If
// more rows are available than the limit requested, the response will indicate
// the next offset of a matching row.
func (u *Server) Match(ctx context.Context, req *MatchReq) (*MatchRsp, error) {
	repo := req.Repository
	if repo != "" {
		repo = tools.FixRepoURL(repo)
	}
	cap := req.Limit
	if cap <= 0 {
		cap = 50 // default limit
	}

	// If a package or package prefix was specified, use that to constrain the
	// scan for an appreciable performance improvement.
	pkg, pfx := req.Package, req.Package
	if t := strings.TrimSuffix(pkg, "/..."); t != pkg {
		pkg, pfx = "", t
	}

	rsp := new(MatchRsp)
	var skipped int
	err := u.graph.Scan(ctx, pfx, func(row *graph.Row) error {
		if (pkg != "" && pkg != row.ImportPath) || (repo != "" && row.Repository != repo) {
			return nil // row does not match
		}
		rsp.NumRows++
		if req.CountOnly {
			// do nothing
		} else if skipped < req.Offset {
			skipped++ // skip prior to the requested offset
		} else if len(rsp.Rows) < cap {
			rsp.Rows = append(rsp.Rows, row)
			rsp.NextOffset = rsp.NumRows
			if !req.IncludeSource {
				row.SourceFiles = nil
			}
			if req.ExcludeDirects {
				row.Directs = nil
			}
		}
		return nil
	})
	if rsp.NumRows == rsp.NextOffset {
		rsp.NextOffset = 0 // we're done here
	}
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

	// Return results starting from the specified offset (0 based).
	Offset int `json:"offset"`
}

// MatchRsp is the response from a successful Match query.
type MatchRsp struct {
	NumRows    int          `json:"numRows"`
	Rows       []*graph.Row `json:"rows,omitempty"`
	NextOffset int          `json:"nextOffset,omitempty"`
}

// Update processes a single update request. An error has concrete type
// *jrpc2.Error and errors during the update phase have a partial response
// attached as a data value.
func (u *Server) Update(ctx context.Context, req *UpdateReq) (*UpdateRsp, error) {
	if req.Repository == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing repository URL")
	} else if req.CheckOnly && req.Force {
		return nil, jrpc2.Errorf(code.InvalidParams, "checkOnly and force are mutually exclusive")
	}
	res, err := u.repoDB.Check(ctx, tools.FixRepoURL(req.Repository))
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
		// TODO: Load options.
		np, err := u.cloneAndUpdate(ctx, res, &u.opts.Options)
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

// Scan performs a scan over all the repositories known to the repo database
// updating each one. Only one scanner is allowed at a time; concurrent calls
// to scan will report an error.
func (u *Server) Scan(ctx context.Context, req *ScanReq) (*ScanRsp, error) {
	if !u.tryScanning() {
		return nil, jrpc2.Errorf(code.SystemError, "scan already in progress")
	}
	defer u.doneScanning()

	rate := req.SampleRate
	if rate == 0 {
		rate = u.opts.SampleRate
	} else if rate < 0 || rate > 1 {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid sampling rate")
	}
	start := time.Now() // for elapsed time
	seen := stringset.New()
	rsp := new(ScanRsp)

	grp, run := taskgroup.New(nil).Limit(u.opts.Concurrency)
	var numPkgs, numRepos int64
	err := u.repoDB.Scan(ctx, "", func(url string) error {
		rsp.NumScanned++

		// Filter duplicates.
		if seen.Contains(url) {
			rsp.NumDups++
			return nil // skip duplicate
		}
		seen.Add(url)

		// Check eligibility.
		stat, err := u.repoDB.Status(ctx, url)
		if err != nil {
			return err // unable to read this record (shouldn't happen)
		} else if !poll.ShouldCheck(stat, u.opts.MinPollInterval) {
			return nil // not eligible for a check yet
		} else if rand.Float64() >= rate {
			return nil // not sampled
		}
		rsp.NumSamples++

		run(func() error {
			// Note that update errors do not fail the scan, but may push back
			// notifications to the client if that is enabled.
			repo, err := u.Update(ctx, &UpdateReq{
				Repository: stat.Repository,
				Reference:  stat.RefName,
			})
			if err == nil {
				atomic.AddInt64(&numPkgs, int64(repo.NumPackages))
			}
			if err != nil {
				u.pushLog(ctx, req.LogErrors, "log.updateError", err)
			} else if repo.NeedsUpdate {
				atomic.AddInt64(&numRepos, 1)
				u.pushLog(ctx, req.LogUpdates, "log.updated", repo)
			} else {
				u.pushLog(ctx, req.LogNonUpdates, "log.skipped", repo)
			}
			return nil
		})
		return nil
	})
	grp.Wait()
	rsp.Elapsed = time.Since(start)
	rsp.NumUpdates = int(numRepos)
	rsp.NumPackages = int(numPkgs)
	return rsp, err
}

// ScanReq is the request parameter to the Scan method.
type ScanReq struct {
	SampleRate    float64 `json:"sampleRate"`    // sampling rate, 0..1; 0 for default
	LogUpdates    bool    `json:"logUpdates"`    // push update notifications
	LogErrors     bool    `json:"logErrors"`     // push error notifications
	LogNonUpdates bool    `json:"logNonUpdates"` // push non-update notifications
}

// ScanRsp is the final result from a successful Scan call.
type ScanRsp struct {
	NumScanned  int `json:"numScanned"`        // number of repositories scanned
	NumDups     int `json:"numDups,omitempty"` // number of duplicates discarded
	NumSamples  int `json:"numSamples"`        // number of samples selected
	NumUpdates  int `json:"numUpdates"`        // number of repositories updated
	NumPackages int `json:"numPackages"`       // number of packages updated

	Elapsed time.Duration `json:"elapsed"` // time elapsed during the scan
}

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
		return tools.FixRepoURL(req.Repositories[i])
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

func (u *Server) pushLog(ctx context.Context, sel bool, key string, arg interface{}) {
	if !sel {
		return
	}
	switch t := arg.(type) {
	case *jrpc2.Error:
		// nothing special
	case error:
		arg = struct {
			E string `json:"message"`
		}{t.Error()}
	}
	u.log(ctx, key, arg)
}
