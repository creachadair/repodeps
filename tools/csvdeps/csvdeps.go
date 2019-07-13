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
	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/tools"
)

var (
	graphDB = flag.String("graph-db", os.Getenv("REPODEPS_DB"), "Graph database")
	repoDB  = flag.String("repo-db", os.Getenv("REPODEPS_POLLDB"), "Repository database")

	useIDFile  = flag.String("ids", "", "Use integer IDs for imports and write them to this file")
	domainOnly = flag.Bool("domain-only", false, "Skip packages without an import domain")
	skipNoDeps = flag.Bool("skip-no-deps", false, "Skip packages without any dependencies")

	pathID = make(map[string]string) // :: import path → id
	idFile = ioutil.Discard
)

func main() {
	flag.Parse()

	s, err := tools.OpenService(*graphDB, *repoDB)
	if err != nil {
		log.Fatalf("Opening service: %v", err)
	}
	defer s.Close()

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

	process := func(row *graph.Row) {
		if _, ok := deps.HasDomain(row.ImportPath); !ok && *domainOnly {
			return
		}
		record := []string{assign(row.ImportPath)}
		for _, dep := range row.Directs {
			if _, ok := deps.HasDomain(dep); !ok && *domainOnly {
				continue
			}
			record = append(record, assign(dep))
		}
		if len(record) > 1 || !*skipNoDeps {
			w.Write(record)
		}
	}

	var nextPage []byte
	for {
		rsp, err := s.Match(ctx, &service.MatchReq{PageKey: nextPage})
		if err != nil {
			log.Fatalf("Match failed: %v", err)
		}
		for _, row := range rsp.Rows {
			process(row)
		}
		nextPage = rsp.NextPage
		if nextPage == nil {
			break
		}
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
