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

// Program listdeps lists the keys of a graph.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
	"github.com/gogo/protobuf/jsonpb"
)

var storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	pfxs := flag.Args()
	if len(pfxs) == 0 {
		pfxs = append(pfxs, "") // list all
	}
	ctx := context.Background()
	var enc jsonpb.Marshaler
	for _, pfx := range pfxs {
		if err := g.Scan(ctx, pfx, func(row *graph.Row) error {
			return enc.Marshal(os.Stdout, row)
		}); err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
	}
}
