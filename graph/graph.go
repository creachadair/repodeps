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

// Package graph defines a storage format for a simple package dependency
// graph. Nodes in the graph are named packages, and edges record direct
// forward dependencies between nodes.
package graph

import (
	"context"
	"fmt"
	"sort"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/storage"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:generate protoc --go_out=. graph.proto

// TODO: Identifiable errors.
// TODO: Reverse index.

// A Graph is an interface to a package dependency graph.
type Graph struct {
	st storage.Interface
}

// New constructs a graph handle for the given storage.
func New(st storage.Interface) *Graph { return &Graph{st: st} }

// Add adds the specified package to the graph. If an entry already exists for
// the specified package, it is replaced.
func (g *Graph) Add(ctx context.Context, repo *deps.Repo, pkg *deps.Package) error {
	var url string
	if len(repo.Remotes) != 0 {
		url = repo.Remotes[0].Url
	}
	var files []*Row_File
	for _, file := range pkg.Sources {
		files = append(files, &Row_File{
			RepoPath: file.RepoPath,
			Digest:   file.Digest,
		})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].RepoPath < files[j].RepoPath
	})
	return g.st.Store(ctx, pkg.ImportPath, &Row{
		Name:        pkg.Name,
		ImportPath:  pkg.ImportPath,
		Repository:  url,
		Directs:     pkg.Imports,
		SourceFiles: files,
		Type:        Row_Type(pkg.Type),
	})
}

// AddAll calls Add for each package defined in the specified repo.
func (g *Graph) AddAll(ctx context.Context, repo *deps.Repo) error {
	for _, pkg := range repo.Packages {
		if err := g.Add(ctx, repo, pkg); err != nil {
			return fmt.Errorf("package %q: %v", pkg.ImportPath, err)
		}
	}
	return nil
}

// Row loads the complete row for the specified import path.
func (g *Graph) Row(ctx context.Context, pkg string) (*Row, error) {
	var row Row
	if err := g.st.Load(ctx, pkg, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

// List calls f with each key in the graph lexicographically greater than or
// equal to start.  If f reports an error, scanning terminates. If the error is
// storage.ErrStopScan, List returns nil. Otherwise, List returns the error
// from f.
func (g *Graph) List(ctx context.Context, start string, f func(string) error) error {
	return g.st.Scan(ctx, start, f)
}

// Scan calls f with each row in the graph whose key is lexicographically
// greater than or equal to start If f reports an error, scanning terminates.
// If the error is storage.ErrStopScan, Scan returns nil. Otherwise Scan
// returns the error from f.
func (g *Graph) Scan(ctx context.Context, start string, f func(*Row) error) error {
	return g.List(ctx, start, func(key string) error {
		row, err := g.Row(ctx, key)
		if err != nil {
			return err
		}
		return f(row)
	})
}

// ScanUpdate calls f with each row in the graph having the specified prefix.
// If f reports true, the modified value of the row is updated; otherwise no
// action is taken for the row.
func (g *Graph) ScanUpdate(ctx context.Context, prefix string, f func(*Row) bool) error {
	return g.Scan(ctx, prefix, func(row *Row) error {
		key := row.ImportPath
		if f(row) {
			return g.st.Store(ctx, key, row)
		}
		return nil
	})
}

// Remove removes the row for pkg from g.
func (g *Graph) Remove(ctx context.Context, pkg string) error {
	return g.st.Delete(ctx, pkg)
}

// MatchImporters calls f(q, p) for each package p that directly depends on any
// package q for which match(q) is true.  The order of results is unspecified.
func (g *Graph) MatchImporters(ctx context.Context, match func(string) bool, f func(tpkg, ipkg string)) error {
	return g.Scan(ctx, "", func(row *Row) error {
		for _, elt := range row.Directs {
			if match(elt) {
				f(elt, row.ImportPath)
				break
			}
		}
		return nil
	})
}

// MarshalJSON implements json.Marshaler for a Row by delegating to protojson.
func (r *Row) MarshalJSON() ([]byte, error) { return protojson.Marshal(r) }

// UnmarshalJSON implements json.Unmarshaler for a Row by delegating to jsonpb.
func (r *Row) UnmarshalJSON(data []byte) error { return protojson.Unmarshal(data, r) }
