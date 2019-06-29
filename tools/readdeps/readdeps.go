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

var storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
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
