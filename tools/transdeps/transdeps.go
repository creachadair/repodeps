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

// Program transdeps lists the transitive closure of forward dependencies of a
// set of packages.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
	"github.com/golang/protobuf/jsonpb"
)

var (
	storePath     = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	noStandardLib = flag.Bool("no-stdlib", false, "Do not visit standard library packages")
)

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var enc jsonpb.Marshaler
	if err := g.DFS(ctx, flag.Args(), func(pkg string, row *graph.Row) error {
		if row == nil {
			log.Printf("[skipping] unindexed dependency %q", pkg)
			return nil
		}
		if *noStandardLib && row.Type == graph.Row_STDLIB {
			return nil
		}
		defer fmt.Println()
		return enc.Marshal(os.Stdout, row)
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}
}
