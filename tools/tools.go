// Package tools implements shared code for command-line tools.
package tools

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/storage"
)

var storePath = flag.String("store", "", "Storage path (required)")

// OpenGraph opens the graph indicated by the -store flag.
// The caller must ensure the closer is closed.
func OpenGraph() (*graph.Graph, io.Closer, error) {
	if *storePath == "" {
		return nil, nil, errors.New("no -store path was provided")
	}
	s, err := badgerstore.NewPath(*storePath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening storage: %v", err)
	}
	return graph.New(storage.NewBlob(s)), s, nil
}
