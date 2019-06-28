// Program readdeps reads the specified rows out of a graph.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/creachadair/repodeps/tools"
)

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph()
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

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
