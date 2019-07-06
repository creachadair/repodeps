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

// Package local analyzes Go dependencies for local Git repositories.
package local

import (
	"context"
	"errors"
	"fmt"
	"go/build"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/tools"
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

	// The local path of the checkout may be inside a GOPATH, in which case we
	// will wind up with the wrong import path. To avoid this, set up a virtual
	// GOPATH containing only this package, in a directory named by the remote
	// URL of the repository.
	repo := &deps.Repo{From: dir, Remotes: remotes}
	repoPrefix := tools.CleanRepoURL(remotes[0].Url)
	vfs := newVFS(dir, repoPrefix)

	var importMode build.ImportMode
	if opts.UseImportComments {
		importMode |= build.ImportComment
	}

	// Find the import paths of the packages defined by this repository, and the
	// import paths of their dependencies. This is basically "go list".
	err = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		} else if !fi.IsDir() {
			return nil // nothing to do here
		} else if deps.IsNonPackage(path) {
			return filepath.SkipDir
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		pkg, err := vfs.buildContext().ImportDir(vfs.fakePath(path), importMode)
		if err != nil {
			return nil // no importable go package here; skip it
		}
		rec := &deps.Package{
			Name:       pkg.Name,
			ImportPath: pkg.ImportPath,
			Imports:    pkg.Imports,
			Type:       deps.PackageType(pkg),
		}
		if opts.UseImportComments {
			if _, ok := deps.HasDomain(pkg.ImportComment); ok {
				rec.ImportPath = pkg.ImportComment
			}
		}
		if opts.TrimRepoPrefix {
			rec.ImportPath = strings.TrimPrefix(rec.ImportPath, repoPrefix+"/")
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
	return tools.FixRepoURL(url)
}

func hashFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return deps.Hash(f), nil
}

// A vfs maps a physical directory to a virtual GOPATH.
type vfs struct {
	realDir string // actual filesystem path
	root    string // advertised filesystem root
}

const separator = string(filepath.Separator)

func newVFS(real, pkg string) vfs {
	return vfs{realDir: real, root: filepath.Join("/go/src", pkg)}
}

// fakePath converts an actual filesystem path to a virtual equivalent.
// If path is not inside the virualized directory, it is unmodified.
func (v vfs) fakePath(path string) string {
	if t := strings.TrimPrefix(path, v.realDir); t != path {
		return filepath.Join(v.root, t)
	}
	return path
}

// fixPath converts a virtual path to its actual filesystem equivalent.
func (v vfs) fixPath(path string) string {
	if t := strings.TrimPrefix(path, v.root); t != path {
		return filepath.Join(v.realDir, t)
	}
	return path
}

func (v vfs) openFile(path string) (io.ReadCloser, error) {
	return os.Open(v.fixPath(path))
}

func (v vfs) readDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(v.fixPath(path))
}

func (v vfs) hasSubdir(root, dir string) (string, bool) {
	if !strings.HasSuffix(root, separator) {
		root += separator
	}
	clean := filepath.Clean(dir)
	if t := strings.TrimPrefix(clean, root); t != clean {
		return t, true
	}
	return "", false
}

func (v vfs) isDir(path string) bool {
	fi, err := os.Stat(v.fixPath(path))
	return err == nil && fi.Mode().IsDir()
}

func (v vfs) buildContext() *build.Context {
	ctx := build.Default
	ctx.GOPATH = "/go"
	ctx.OpenFile = v.openFile
	ctx.ReadDir = v.readDir
	ctx.HasSubdir = v.hasSubdir
	ctx.IsDir = v.isDir
	return &ctx
}
