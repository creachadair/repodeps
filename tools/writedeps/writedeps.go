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

// Program writedeps copies a stream of JSON-encoded *deps.Repo messages into a
// graph in adjacency list format.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/fileinput"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/tools"
)

var storePath = flag.String("store", "", "Storage path (required)")

func main() {
	flag.Parse()

	g, c, err := tools.OpenGraph(*storePath)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}

	ctx := context.Background()
	rc := fileinput.CatOrFile(ctx, flag.Args(), os.Stdin)
	defer rc.Close()
	dec := json.NewDecoder(rc)
	for dec.More() {
		var msg []*deps.Repo
		if err := dec.Decode(&msg); err != nil {
			log.Fatalf("Decoding failed: %v", err)
		}
		for _, repo := range msg {
			for _, pkg := range repo.Packages {
				if err := g.Add(ctx, repo, pkg); err != nil {
					log.Fatalf("Adding package %q: %v", pkg.ImportPath, err)
				}
				fmt.Println(pkg.ImportPath)
			}
		}
	}

	if err := c.Close(); err != nil {
		log.Fatalf("Closing storage: %v", err)
	}
}
