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

// Package siva analyzes Go dependences for rooted Git repositories packed into
// siva archives by github.com/src-d/borges.
package siva

import (
	"context"
	"fmt"
	"go/build"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/tools"

	sivafs "gopkg.in/src-d/go-billy-siva.v4"
	"gopkg.in/src-d/go-billy.v4/osfs"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/cache"
	"gopkg.in/src-d/go-git.v4/plumbing/filemode"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/filesystem"
)

// Load reads the repository structure of a Siva archive file.  This may return
// multiple repositories, if the file is rooted.
func Load(ctx context.Context, path string, opts *deps.Options) ([]*deps.Repo, error) {
	check := func(op string) error {
		select {
		case <-ctx.Done():
			return fmt.Errorf("%s: %v", op, ctx.Err())
		default:
			return nil
		}
	}
	if opts == nil {
		opts = new(deps.Options)
	}
	fs := osfs.New("/")
	sfs, err := sivafs.NewFilesystemReadOnly(fs, path, 0)
	if err != nil {
		return nil, fmt.Errorf("opening siva filesystem: %v", err)
	}

	// N.B.: The cache parameter must be non-nil for any task where object
	// contents must be read.
	stg := filesystem.NewStorage(sfs, cache.NewObjectLRUDefault())
	repo, err := git.Open(stg, nil)
	if err != nil {
		return nil, fmt.Errorf("opening git repo from %q: %v", path, err)
	}
	cfg, err := repo.Config()
	if err != nil {
		return nil, fmt.Errorf("reading config: %v", err)
	}

	// Record the mapping between UUID and repository, so we can dispatch the
	// package mappings correctly.
	var results []*deps.Repo
	repos := make(map[string]*deps.Repo)
	for _, rem := range cfg.Remotes {
		r := &deps.Repo{
			From: path,
			Remotes: []*deps.Remote{{
				Name: rem.Name,
				Url:  tools.FixRepoURL(rem.URLs[0]),
			}},
		}
		repos[rem.Name] = r
		results = append(results, r)
	}

	// Siva files generated by Borges are usually "rooted", meaning that there
	// may be multiple repositories sharing a common history prefix.
	//
	// In a rooted repository there may be no single HEAD; the references point
	// to the tips of the repositories that are packed together.  We must
	// therefore visit all the references and walk through their tip commits to
	// find the current state of the repositories that are merged here.  We pick
	// the first match for each repo UUID.
	refs, err := repo.References()
	if err != nil {
		return nil, fmt.Errorf("listing references: %v", err)
	}

	var cur string
	if err := refs.ForEach(func(ref *plumbing.Reference) error {
		if err := check("next reference"); err != nil {
			return err
		}
		// Rooted references have the form REFNAME/REMOTE. Skip refs that don't
		// look like this.
		name := string(ref.Name())
		if i := strings.LastIndex(name, "/"); i < 0 {
			return nil // skip un-rooted reference
		} else if uuid := name[i+1:]; uuid == cur {
			return nil // we already have a ref for this repo
		} else {
			cur = uuid
		}

		// Load the tree for the tip comment and scan its files.
		here := repos[cur]
		comm, err := repo.CommitObject(ref.Hash())
		if err != nil {
			return fmt.Errorf("fetching commit: %v", err)
		}
		tree, err := comm.Tree()
		if err != nil {
			return fmt.Errorf("fetching tree: %v", err)
		}

		// Record the directory structure to support the build.Context VFS.
		vfs := newVFS(tools.CleanRepoURL(here.Remotes[0].Url))
		if err := tree.Files().ForEach(func(f *object.File) error {
			if err := check("scanning files"); err != nil {
				return err
			} else if !deps.IsVendor(f.Name) {
				vfs.add(f)
			}
			return nil
		}); err != nil {
			return err
		}

		var importMode build.ImportMode
		if opts.UseImportComments {
			importMode |= build.ImportComment
		}
		bc := vfs.buildContext()
		for dir := range vfs.dirs {
			if err := check("importing packages"); err != nil {
				return err
			}
			pkg, err := bc.ImportDir(dir, importMode)
			if err != nil {
				continue // no importable go package here; skip it
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
			if opts.HashSourceFiles {
				for _, name := range pkg.GoFiles {
					fpath := filepath.Join(dir, name)
					r, err := vfs.open(fpath)
					if err != nil {
						return fmt.Errorf("reading file: %v", err)
					}
					rec.Sources = append(rec.Sources, &deps.File{
						RepoPath: vfs.rel(here.Remotes[0].Url, fpath),
						Digest:   deps.Hash(r),
					})
					r.Close()
				}
			}
			here.Packages = append(here.Packages, rec)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("scanning references: %v", err)
	}
	return results, nil
}

// vfile wraps a go-git File object to implement the os.FileInfo interface.
type vfile struct {
	f *object.File
}

func (v vfile) Name() string       { return filepath.Base(v.f.Name) }
func (v vfile) Size() int64        { return v.f.Blob.Size }
func (v vfile) Mode() os.FileMode  { return os.FileMode(v.f.Mode) }
func (v vfile) IsDir() bool        { return v.f.Mode&filemode.Dir != 0 }
func (v vfile) ModTime() time.Time { return time.Time{} }
func (vfile) Sys() interface{}     { return nil }

// vfs tracks a virtual directory structure from the flattened contents of a
// file listing in a Git repository, and exports accessors to support the VFS
// used by the go/build package.
type vfs struct {
	// To get the right import path we have to convince the build package that
	// the files are stored in a well-known relationship to the GOPATH.  We do
	// this by prefixing each path with "/src/<url>", e.g., if the URL for the
	// remote is github.com/foo/bar, this is /src/github.com/foo/bar.  During
	// import, we use "/" as the GOPATH.
	prefix string

	files map[string]vfile    // :: path → file
	dirs  map[string][]string // :: path → [name]
}

func newVFS(root string) *vfs {
	return &vfs{
		prefix: filepath.Join("/src", root),
		files:  make(map[string]vfile),
		dirs:   make(map[string][]string),
	}
}

func (v *vfs) rel(url, path string) string {
	rel, _ := filepath.Rel(filepath.Join("/src", url), path)
	return rel
}

func (v *vfs) add(f *object.File) {
	dir, name := v.prefix, f.Name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		dir, name = filepath.Join(v.prefix, name[:i]), name[i+1:]
	}
	if deps.IsNonPackage(dir) {
		return // don't bother with this one
	}
	v.files[filepath.Join(v.prefix, f.Name)] = vfile{f}
	v.dirs[dir] = append(v.dirs[dir], name)
}

func (v *vfs) buildContext() build.Context {
	ctx := build.Default
	ctx.GOPATH = "/"
	ctx.IsDir = v.isDir
	ctx.ReadDir = v.readDir
	ctx.OpenFile = v.open
	return ctx
}

func (v *vfs) open(path string) (io.ReadCloser, error) {
	f, ok := v.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	return f.f.Blob.Reader()
}

func (v *vfs) readDir(path string) ([]os.FileInfo, error) {
	lst, ok := v.dirs[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	var out []os.FileInfo
	for _, name := range lst {
		f, ok := v.files[filepath.Join(path, name)]
		if ok {
			out = append(out, f)
		}
	}
	return out, nil
}

func (v *vfs) isDir(path string) bool {
	_, ok := v.dirs[path]
	return ok
}
