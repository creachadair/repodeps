// Program revdeps lists the reverse dependencies of a package.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

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
	for _, pkg := range flag.Args() {
		if err := g.Importers(ctx, pkg, func(ipath string) {
			fmt.Println(ipath)
		}); err != nil {
			log.Fatalf("Importers failed: %v", err)
		}
	}
}
