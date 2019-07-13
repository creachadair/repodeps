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

// Program listdeps lists the keys and values of a graph.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/tools"
)

var (
	graphDB = flag.String("graph-db", os.Getenv("REPODEPS_DB"), "Graph database")
	repoDB  = flag.String("repo-db", os.Getenv("REPODEPS_POLLDB"), "Repository database")

	doKeysOnly  = flag.Bool("keys", false, "Print only import paths, not full rows")
	doFilterDom = flag.Bool("domain-only", false, "Print only import paths that begin with a domain")
	matchRepo   = flag.String("repo", "", "List only rows matching this repository")
)

func main() {
	flag.Parse()

	s, err := tools.OpenService(*graphDB, *repoDB)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	enc := json.NewEncoder(os.Stdout)
	var pkg string
	if flag.NArg() != 0 {
		pkg = flag.Arg(0)
	}

	var nextPage []byte
	for {
		rsp, err := s.Match(ctx, &service.MatchReq{
			Package:    pkg,
			Repository: *matchRepo,
			PageKey:    nextPage,
		})
		if err != nil {
			log.Printf("Listing %q failed: %v", pkg, err)
			break
		}
		for _, row := range rsp.Rows {
			if _, ok := deps.HasDomain(row.ImportPath); !ok && *doFilterDom {
				continue
			} else if *doKeysOnly {
				fmt.Println(row.ImportPath)
			} else {
				enc.Encode(row)
			}
		}
		nextPage = rsp.NextPage
		if nextPage == nil {
			break
		}
	}
}
