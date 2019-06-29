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

// Program revdeps lists the reverse dependencies of a package.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var (
	storePath   = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	matchPerfix = flag.Bool("prefix", false, "Prefix-based matching")
	printDirect = flag.Bool("direct", false, "Print direct dependencies as well")
)

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraphReadOnly(*storePath)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	var importers func(context.Context, string, func(*graph.Row, int)) error
	if *matchPerfix {
		importers = g.ImportersScan
	} else {
		importers = g.Importers
	}

	ctx := context.Background()
	for _, pkg := range flag.Args() {
		if err := importers(ctx, pkg, func(row *graph.Row, i int) {
			if *printDirect {
				fmt.Printf("%s ", row.Directs[i])
			}
			fmt.Println(row.ImportPath)
		}); err != nil {
			log.Fatalf("Importers failed: %v", err)
		}
	}
}
