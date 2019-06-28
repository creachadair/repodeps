// Program listdeps lists the keys of a graph.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"strings"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph()
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	pfxs := flag.Args()
	if len(pfxs) == 0 {
		pfxs = append(pfxs, "") // list all
	}
	ctx := context.Background()
	for _, pfx := range pfxs {
		if err := g.Scan(ctx, pfx, func(row *graph.Row) error {
			fmt.Printf("%s,%s\n", row.ImportPath, strings.Join(row.Directs, ","))
			return nil
		}); err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
	}
}
