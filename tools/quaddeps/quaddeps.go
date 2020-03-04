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

// Program quaddeps compiles a graph into RDF triples.
package main

import (
	"context"
	"flag"
	"log"
	"os"

	"github.com/cayleygraph/cayley"
	"github.com/cayleygraph/cayley/graph"
	"github.com/cayleygraph/cayley/graph/kv/bolt"
	"github.com/creachadair/repodeps/tools"
)

var (
	graphDB    = flag.String("graph-db", os.Getenv("DEPSERVER_DB"), "Graph database path (required)")
	outputPath = flag.String("output", "", "Output storage path (optional)")
)

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*graphDB)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	var werr error
	if *outputPath != "" {
		if err := graph.InitQuadStore(bolt.Type, *outputPath, nil); err != nil {
			log.Fatalf("Initializing output: %v", err)
		}
		st, err := cayley.NewGraph(bolt.Type, *outputPath, nil)
		if err != nil {
			log.Fatalf("Opening output: %v", err)
		}
		defer func() {
			if err := st.Close(); err != nil {
				log.Fatalf("Closing output: %v", err)
			}
		}()
		werr = g.EncodeToQuads(ctx, st.QuadWriter.AddQuad)
	} else {
		werr = g.WriteQuads(ctx, os.Stdout)
	}
	if werr != nil {
		log.Fatalf("Writing output: %v", err)
	}
}
