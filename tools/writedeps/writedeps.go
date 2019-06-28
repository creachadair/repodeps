// Program writedeps copies a stream of JSON-encoded *deps.Repo messages into a
// graph in adjacency list format.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/fileinput"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/golang/protobuf/proto"
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
	g := graph.New(storage{s})

	ctx := context.Background()
	rc := fileinput.CatOrFile(ctx, flag.Args(), os.Stdin)
	defer rc.Close()
	dec := json.NewDecoder(rc)
	for dec.More() {
		var msg []*deps.Repo
		if err := dec.Decode(&msg); err != nil {
			log.Fatalf("Decoding failed: %v", err)
		}
		for _, repo := range msg {
			for _, pkg := range repo.Packages {
				if err := g.Add(ctx, pkg); err != nil {
					log.Fatalf("Adding package %q: %v", pkg.ImportPath, err)
				}
				fmt.Println(pkg.ImportPath)
			}
		}
	}
	if err := s.Close(); err != nil {
		log.Fatalf("Closing storage: %v", err)
	}
}

type storage struct {
	bs *badgerstore.Store
}

func (s storage) Load(ctx context.Context, key string, val proto.Message) error {
	bits, err := s.bs.Get(ctx, key)
	if err != nil {
		return err
	}
	return proto.Unmarshal(bits, val)
}

func (s storage) Store(ctx context.Context, key string, val proto.Message) error {
	bits, err := proto.Marshal(val)
	if err != nil {
		return err
	}
	return s.bs.Put(ctx, blob.PutOptions{
		Key:     key,
		Data:    bits,
		Replace: true,
	})
}
