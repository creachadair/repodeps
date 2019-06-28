// Program readdeps reads the specified rows out of a graph.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/storage"
)

var (
	storePath = flag.String("store", "", "Storage path (required)")
)

func main() {
	flag.Parse()
	switch {
	case *storePath == "":
		log.Fatal("You must provide a non-empty -store path")
	}

	s, err := badgerstore.NewPath(*storePath)
	if err != nil {
		log.Fatalf("Opening storage: %v", err)
	}
	defer s.Close()
	g := graph.New(storage.NewBlob(s))

	ctx := context.Background()
	enc := json.NewEncoder(os.Stdout)
	for _, ipath := range flag.Args() {
		row, err := g.Row(ctx, ipath)
		if err != nil {
			log.Printf("Reading %q: %v", ipath, err)
			continue
		}
		if err := enc.Encode(row); err != nil {
			log.Fatalf("Writing output: %v", err)
		}
	}
}
