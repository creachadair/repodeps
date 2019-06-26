package siva

import (
	"context"
	"fmt"
	"go/build"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/creachadair/repodeps/deps"

	sivafs "gopkg.in/src-d/go-billy-siva.v4"
	"gopkg.in/src-d/go-billy.v4/memfs"
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
func Load(_ context.Context, path string, opts *deps.Options) ([]*deps.Repo, error) {
	if opts == nil {
		opts = new(deps.Options)
	}
	fs := osfs.New("/")
	sfs, err := sivafs.NewFilesystem(fs, path, memfs.New())
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
			Remotes: []*deps.Remote{{
				Name: rem.Name,
				URL:  fixURL(rem.URLs[0]),
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
			return err
		}
		tree, err := comm.Tree()
		if err != nil {
			return err
		}

		// Record the directory structure to support the build.Context VFS.
		vfs := newVFS()
		if err := tree.Files().ForEach(func(f *object.File) error {
			if !deps.IsVendor(f.Name) {
				vfs.add(f)
			}
			return nil
		}); err != nil {
			return err
		}

		bc := vfs.buildContext()
		for path := range vfs.dirs {
			pkg, err := bc.ImportDir(path, 0)
			if err != nil {
				log.Printf("[skipping] %v", err)
				continue
			}
			rec := &deps.Package{
				Name:       pkg.Name,
				ImportPath: pkg.ImportPath,
				Imports:    pkg.Imports,
			}
			if opts.HashSourceFiles {
				for _, name := range pkg.GoFiles {
					path := filepath.Join(path, name)
					r, err := vfs.open(path)
					if err != nil {
						return fmt.Errorf("reading file: %v", err)
					}
					rec.Source = append(rec.Source, &deps.File{
						Name:   name,
						Digest: deps.Hash(r),
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
	files map[string]vfile    // :: path → file
	dirs  map[string][]string // :: path → [name]
}

func newVFS() *vfs {
	return &vfs{
		files: make(map[string]vfile),
		dirs:  make(map[string][]string),
	}
}

func (v *vfs) add(f *object.File) {
	dir, name := "/", f.Name
	if i := strings.LastIndex(name, "/"); i >= 0 {
		dir, name = "/"+name[:i], name[i+1:]
	}
	v.files["/"+f.Name] = vfile{f}
	v.dirs[dir] = append(v.dirs[dir], name)
}

func (v *vfs) buildContext() build.Context {
	ctx := build.Default
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

func fixURL(s string) string {
	s = strings.TrimSuffix(s, ".git")
	if trim := strings.TrimPrefix(s, "git://"); trim != s {
		return "https://" + trim
	}
	return s
}
