package poll

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes"
)

// CheckResult records the update status of a repository.
type CheckResult struct {
	URL    string // repository fetch URL
	Name   string // remote head name
	Digest string // current digest value
	Errors int    // errors since last successful update

	old string // old digest value
}

// NeedsUpdate reports whether c requires an update.
func (c *CheckResult) NeedsUpdate() bool { return c.old != c.Digest }

// Clone clones the repository state denoted by c in specified directory path.
// The directory is created if it does not exist.
func (c *CheckResult) Clone(ctx context.Context, path string) error {
	dir, base := filepath.Split(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	cmd := git(ctx, "-C", dir, "clone", "--no-checkout", "--depth=1", c.URL, base)
	if _, err := cmd.Output(); err != nil {
		return runErr(err)
	}
	_, err := git(ctx, "-C", path, "checkout", "--detach", c.Digest).Output()
	return runErr(err)
}

// ShouldCheck reports whether the given status message should be checked,
// based on its history of previous updates. No update will be suggested within
// min of the most recent check. Otherwise, schedule an update once the current
// time is at least the average gap between updates.
func ShouldCheck(stat *Status, min time.Duration) bool {
	now := time.Now()
	then, _ := ptypes.Timestamp(stat.LastCheck)

	// Do not do an update within min after the last update.
	if then.Add(min).After(now) {
		return false
	}

	n := len(stat.Updates)
	if n == 0 {
		return true
	}
	// Compute the average time between updates and schedule one if it has been
	// at least that long since the last.
	first, _ := ptypes.Timestamp(stat.Updates[0].When)
	last, _ := ptypes.Timestamp(stat.Updates[n-1].When)
	avg := last.Sub(first) / time.Duration(n)
	return then.Add(avg).Before(now)
}
