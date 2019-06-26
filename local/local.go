package local

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creachadair/repodeps/deps"
)

// Load reads the repository structure of a local directory. This will return
// only a single repository, since local repositories are not rooted.
func Load(ctx context.Context, dir string, opts *deps.Options) ([]*deps.Repo, error) {
	if opts == nil {
		opts = new(deps.Options)
	}
	// Find the URLs for the remotes defined for this repository.
	remotes, err := gitRemotes(ctx, dir)
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %v", err)
	} else if len(remotes) == 0 {
		return nil, errors.New("no remotes defined")
	}

	repo := &deps.Repo{From: dir, Remotes: remotes}

	// Find the import paths of the packages defined by this repository, and the
	// import paths of their dependencies. This is basically "go list".
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !fi.IsDir() {
			return nil // nothing to do here
		} else if base := filepath.Base(path); base == ".git" || base == "vendor" {
			return filepath.SkipDir
		}
		pkg, err := build.Default.ImportDir(path, 0)
		if err != nil {
			log.Printf("[skipping] %v", err)
			return nil
		}
		rec := &deps.Package{
			Name:       pkg.Name,
			ImportPath: pkg.ImportPath,
			Imports:    pkg.Imports,
		}
		if opts.HashSourceFiles {
			for _, name := range pkg.GoFiles {
				fpath := filepath.Join(path, name)
				hash, err := hashFile(fpath)
				if err != nil {
					log.Printf("Hashing %q failed: %v", path, err)
				}
				rel, _ := filepath.Rel(dir, fpath)
				rec.Sources = append(rec.Sources, &deps.File{
					RepoPath: rel,
					Digest:   hash,
				})
			}
		}
		repo.Packages = append(repo.Packages, rec)
		return nil
	})
	return []*deps.Repo{repo}, err
}

func gitRemotes(ctx context.Context, dir string) ([]*deps.Remote, error) {
	cmd := exec.CommandContext(ctx, "git", "remote")
	cmd.Dir = dir
	bits, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing remotes: %v", err)
	}

	names := strings.Split(strings.TrimSpace(string(bits)), "\n")
	var rs []*deps.Remote
	for _, name := range names {
		cmd := exec.CommandContext(ctx, "git", "remote", "get-url", name)
		cmd.Dir = dir
		bits, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("getting remote URL for %q: %v", name, err)
		}
		rs = append(rs, &deps.Remote{Name: name, Url: parseRemote(bits)})
	}
	return rs, nil
}

func parseRemote(bits []byte) string {
	url := strings.TrimSpace(string(bits))
	if trim := strings.TrimPrefix(url, "git@"); trim != url {
		parts := strings.SplitN(trim, ":", 2)
		url = parts[0]
		if len(parts) == 2 {
			url += "/" + parts[1]
		}
	}
	return strings.TrimSuffix(url, ".git")
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return deps.Hash(f), nil
}
