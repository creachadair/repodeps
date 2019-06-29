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

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	have := stringset.New()
	want := stringset.New()
	if err := g.Scan(ctx, "", func(row *graph.Row) error {
		have.Add(row.ImportPath)
		want.Add(row.Directs...)
		return nil
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}
	for pkg := range want.Diff(have) {
		fmt.Println(pkg)
	}
}
