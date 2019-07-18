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

	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/tools"
)

var (
	graphDB = flag.String("graph-db", os.Getenv("REPODEPS_DB"), "Graph database")
	repoDB  = flag.String("repo-db", os.Getenv("REPODEPS_POLLDB"), "Repository database")
)

func main() {
	flag.Parse()

	s, err := tools.OpenService(*graphDB, *repoDB)
	if err != nil {
		log.Fatalf("Opening service: %v", err)
	}
	defer s.Close()

	ctx := context.Background()
	enc := json.NewEncoder(os.Stdout)
	for _, ipath := range flag.Args() {
		rsp, err := s.Match(ctx, &service.MatchReq{
			Package:      ipath,
			IncludeFiles: true,
		})
		if err != nil {
			log.Printf("Reading %q: %v", ipath, err)
		} else if rsp.NumRows == 0 {
			log.Printf("Package %q not found", ipath)
		} else {
			for _, row := range rsp.Rows {
				enc.Encode(row)
			}
		}
	}
}
