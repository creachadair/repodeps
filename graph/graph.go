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
	"errors"
	"sort"
	"strings"

	"github.com/creachadair/repodeps/deps"
	"github.com/golang/protobuf/proto"
)

//go:generate protoc --go_out=. graph.proto

// TODO: Identifiable errors.
// TODO: Reverse index.

// A Graph is an interface to a package dependency graph.
type Graph struct {
	st Storage
}

// New constructs a graph handle for the given storage.
func New(st Storage) *Graph { return &Graph{st: st} }

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

// Row loads the complete row for the specified import path.
func (g *Graph) Row(ctx context.Context, pkg string) (*Row, error) {
	var row Row
	if err := g.st.Load(ctx, pkg, &row); err != nil {
		return nil, err
	}
	return &row, nil
}

// List calls f with each key in the graph having the specified prefix.
// If f reports an error, scanning terminates. If te error is ErrStopScan,
// List returns nil. Otherwise, List returns the error from f.
func (g *Graph) List(ctx context.Context, prefix string, f func(string) error) error {
	err := g.st.Scan(ctx, prefix, f)
	if err == ErrStopScan {
		return nil
	}
	return err
}

// Scan calls f with each row in the graph having the specified prefix.
// If f reports an error, scanning terminates. If the error is ErrStopScan Scan
// returns nil; otherwise Scan returns the error from f.
func (g *Graph) Scan(ctx context.Context, prefix string, f func(*Row) error) error {
	return g.List(ctx, prefix, func(key string) error {
		row, err := g.Row(ctx, key)
		if err != nil {
			return err
		}
		return f(row)
	})
}

// Imports returns the import paths if the direct dependencies of pkg.
func (g *Graph) Imports(ctx context.Context, pkg string) ([]string, error) {
	row, err := g.Row(ctx, pkg)
	if err != nil {
		return nil, err
	}
	return row.Directs, nil
}

// Importers calls f for each package that directly depends on pkg.
// If pkg ends with "/...", any package with that prefix is matched.  For
// example "regexp/..." matches "regexp" and "regexp/syntax".
// The order of results is unspecified.
func (g *Graph) Importers(ctx context.Context, pkg string, f func(tpkg, ipkg string)) error {
	if t := strings.TrimSuffix(pkg, "/..."); t != pkg {
		return g.MatchImporters(ctx, func(s string) bool { return strings.HasPrefix(s, t) }, f)
	}
	return g.MatchImporters(ctx, func(s string) bool { return s == pkg }, f)
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

// Storage represents the interface to persistent storage.
type Storage interface {
	// Load reads the data for the specified key and unmarshals it into val.
	Load(ctx context.Context, key string, val proto.Message) error

	// Store marshals the data from value and stores it under key.
	Store(ctx context.Context, key string, val proto.Message) error

	// Scan calls f with each key having the specified prefix. If f reports an
	// error that error is propagated to the caller of Scan.
	Scan(ctx context.Context, prefix string, f func(string) error) error
}

// ErrStopScan is returned by the callback to Scan to signal that scanning
// should terminate without error.
var ErrStopScan = errors.New("stop scanning")
