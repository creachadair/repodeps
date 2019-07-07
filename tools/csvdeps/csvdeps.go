// Copyright 2019 Sourced LLC. All Rights Reserved.
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

// Program csv export graph into a CSV in adjacency list format ready to load
// in Gephi.
//
// See https://gephi.org/users/supported-graph-formats/csv-format
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

var (
	storePath  = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	useIDFile  = flag.String("ids", "", "Use integer IDs for imports and write them to this file")
	domainOnly = flag.Bool("domain-only", false, "Skip packages without an import domain")
	skipNoDeps = flag.Bool("skip-no-deps", false, "Skip packages without any dependencies")

	pathID = make(map[string]string) // :: import path → id
	idFile = ioutil.Discard
)

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	if *useIDFile != "" {
		f, err := os.Create(*useIDFile)
		if err != nil {
			log.Fatalf("Creating ID file: %v", err)
		}
		idFile = f
		defer func() {
			if err := f.Close(); err != nil {
				log.Fatalf("Closing ID file: %v", err)
			}
		}()
		log.Printf("Writing ID vocabulary to %q", *useIDFile)
	}

	ctx := context.Background()
	w := csv.NewWriter(os.Stdout)
	w.Comma = ';'
	defer w.Flush()

	process := func(_ string, row *graph.Row) error {
		if row == nil {
			return nil
		} else if _, ok := deps.HasDomain(row.ImportPath); !ok && *domainOnly {
			return nil
		}

		record := []string{assign(row.ImportPath)}
		for _, dep := range row.Directs {
			if _, ok := deps.HasDomain(dep); !ok && *domainOnly {
				continue
			}
			record = append(record, assign(dep))
		}
		if len(record) == 1 && *skipNoDeps {
			return nil
		}
		return w.Write(record)
	}

	if flag.NArg() == 0 {
		err = g.Scan(ctx, "", func(row *graph.Row) error {
			return process(row.ImportPath, row)
		})
	} else {
		err = g.DFS(ctx, flag.Args(), process)
	}
	if err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	log.Printf("Found %d unique nodes", len(pathID))
}

func assign(path string) string {
	if *useIDFile == "" {
		pathID[path] = "" // count only
		return path
	}
	id, ok := pathID[path]
	if !ok {
		id = fmt.Sprintf("%d", len(pathID)+1)
		pathID[path] = id
		fmt.Fprintln(idFile, id, path)
	}
	return id
}
