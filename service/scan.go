package service

import (
	"context"
	"errors"
	"math/rand"
	"sync/atomic"
	"time"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/taskgroup"
)

// Scan performs a scan over all the repositories known to the repo database
// updating each one. Only one scanner is allowed at a time; concurrent calls
// to scan will report an error.
func (u *Server) Scan(ctx context.Context, req *ScanReq) (*ScanRsp, error) {
	if u.opts.ReadOnly {
		return nil, errors.New("database is read-only")
	} else if !u.tryScanning() {
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
	err := u.repoDB.Scan(ctx, func(url string) error {
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

func (u *Server) tryScanning() bool {
	return atomic.AddInt32(&u.scanning, 1) == 1
}

func (u *Server) doneScanning() { atomic.StoreInt32(&u.scanning, 0) }
