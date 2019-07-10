// Package updater defines a service that maintains the state of a dependency
// graph. It is compatible with the github.com/creachadair/jrpc2 package, but
// can also be used independently.
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
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
)

// Options control the behaviour of an Updater.
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

	// Default package loader options.
	deps.Options
}

// New constructs a new Updater from the specified options.  As long as the
// updater is open, it holds a lock on the databases assigned to it.  The
// caller must call Close when the updater is no longer in use.
func New(opts Options) (*Updater, error) {
	if opts.RepoDB == "" {
		return nil, errors.New("no repository database")
	}
	if opts.GraphDB == "" {
		return nil, errors.New("no graph database")
	}
	if opts.SampleRate == 0 {
		opts.SampleRate = 1
	}
	u := &Updater{opts: opts}
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

// An Updater manages updates to a database of dependencies.
type Updater struct {
	repoDB *poll.DB
	repoC  io.Closer
	graph  *graph.Graph
	graphC io.Closer

	opts Options
}

// Close shuts down the updater and closes its underlying data stores.
func (u *Updater) Close() error {
	rerr := u.repoC.Close()
	gerr := u.graphC.Close()
	if rerr != nil {
		return rerr
	}
	return gerr
}

// Update processes a single update request. An error has concrete type
// *jrpc2.Error and errors during the update phase have a partial response
// attached as a data value.
func (u *Updater) Update(ctx context.Context, req *UpdateReq) (*UpdateRsp, error) {
	if req.Repository == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "missing repository URL")
	}
	res, err := u.repoDB.Check(ctx, tools.FixRepoURL(req.Repository))
	if err != nil {
		return nil, jrpc2.Errorf(code.SystemError, "checking %q: %v", req.Repository, err)
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
			e := jrpc2.DataErrorf(code.SystemError, out, "update %q: %v", res.URL, err)
			log.Printf("MJF :: e=%#v", e)
			return nil, e
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
// updating each one.
func (u *Updater) Scan(ctx context.Context, req *ScanReq) (*ScanRsp, error) {
	rate := req.SampleRate
	if rate == 0 {
		rate = u.opts.SampleRate
	} else if rate < 0 || rate > 1 {
		return nil, jrpc2.Errorf(code.InvalidParams, "invalid sampling rate")
	}
	start := time.Now() // for elapsed time
	seen := stringset.New()
	rsp := new(ScanRsp)
	err := u.repoDB.Scan(ctx, "", func(url string) error {
		rsp.NumScanned++

		// Filter duplicates.
		url = tools.FixRepoURL(url)
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

		// Note that update errors do not fail the scan, but may push back
		// notifications to the client if that is enabled.
		repo, err := u.Update(ctx, &UpdateReq{
			Repository: stat.Repository,
			Reference:  stat.RefName,
		})
		if err != nil {
			u.pushLog(ctx, req.LogErrors, "log.updateError", err)
		} else if repo.NeedsUpdate {
			u.pushLog(ctx, req.LogUpdates, "log.updated", repo)
		} else {
			u.pushLog(ctx, req.LogNonUpdates, "log.skipped", repo)
		}
		return nil
	})
	rsp.Elapsed = time.Since(start)
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
	NumPackages int `json:"numPackages"`       // number of packages updated

	Elapsed time.Duration `json:"elapsed"` // time elapsed during the scan
}

func (u *Updater) cloneAndUpdate(ctx context.Context, res *poll.CheckResult, opts *deps.Options) (int, error) {
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

func (u *Updater) pushLog(ctx context.Context, sel bool, key string, arg interface{}) {
	if !sel {
		return
	} else if e, ok := arg.(*jrpc2.Error); ok {
		var data json.RawMessage
		e.UnmarshalData(&data)
		arg = []interface{}{e.Message(), data}
	}
	jrpc2.ServerPush(ctx, key, arg)
}
