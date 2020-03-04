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

// Program missingdeps scans a graph database for keys that are listed as
// dependencies but whose dependencies are not known.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
)

var (
	address = flag.String("address", os.Getenv("DEPSERVER_ADDR"), "Service address")

	doFilterDom = flag.Bool("domain-only", false, "Print only import paths that begin with a domain")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options]

Scan the contents of a dependency graph to find import paths mentioned by one
or more packages in the graph that do not have a corresponding graph node.

By default, all missing import paths are printed. With -domain-only, only
import paths having the form "host.dom/path/to/pkg" are considered. This filter
eliminates packages accessed via custom build hooks, as well as the standard
library.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()

	ctx := context.Background()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	have := stringset.New()
	want := stringset.New()
	nr, err := c.Match(ctx, new(service.MatchReq), func(row *graph.Row) error {
		have.Add(row.ImportPath)
		want.Add(row.Directs...)
		return nil
	})
	if err != nil {
		log.Fatalf("Match failed: %v", err)
	}
	log.Printf("Matched %d rows", nr)

	for pkg := range want.Diff(have) {
		_, ok := deps.HasDomain(pkg)
		if ok || !*doFilterDom {
			fmt.Println(pkg)
		}
	}
}
