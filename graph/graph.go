// Package graph defines a storage format for a package dependency graph.
package graph

import (
	"context"

	"github.com/creachadair/repodeps/deps"
	"github.com/golang/protobuf/proto"
)

//go:generate protoc --go_out=. graph.proto

// TODO: Identifiable errors.
// TODO: Reverse index.
// TODO: RDF output.

// A Graph is an interface to a package dependency graph.
type Graph struct {
	st Storage
}

// New constructs a graph handle for the given storage.
func New(st Storage) *Graph { return &Graph{st: st} }

// Add adds the specified package to the graph.
func (g *Graph) Add(ctx context.Context, pkg *deps.Package) error {
	return g.st.Store(ctx, pkg.ImportPath, &Row{
		Name:       pkg.Name,
		ImportPath: pkg.ImportPath,
		Directs:    pkg.Imports,
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

// Imports returns the import paths if the direct dependencies of pkg.
func (g *Graph) Imports(ctx context.Context, pkg string) ([]string, error) {
	row, err := g.Row(ctx, pkg)
	if err != nil {
		return nil, err
	}
	return row.Directs, nil
}

// Storage represents the interface to persistent storage.
type Storage interface {
	// Load reads the data for the specified key and unmarshals it into val.
	Load(ctx context.Context, key string, val proto.Message) error

	// Store marshals the data from value and stores it under key.
	Store(ctx context.Context, key string, val proto.Message) error
}
