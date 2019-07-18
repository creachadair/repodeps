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
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
)

var (
	address = flag.String("address", os.Getenv("REPODEPS_ADDR"), "Service address")

	doCountOnly  = flag.Bool("count", false, "Count the number of matching packages")
	doKeysOnly   = flag.Bool("keys", false, "Print only import paths, not full rows")
	rowLimit     = flag.Int("limit", 0, "List at most this many matching rows (0 = no limit)")
	matchPackage = flag.String("pkg", "", "Match this package or prefix with /...")
	matchRepo    = flag.String("repo", "", "List only rows matching this repository")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	if *matchPackage == "" && flag.NArg() != 0 {
		*matchPackage = flag.Arg(0)
	}

	enc := json.NewEncoder(os.Stdout)
	nr, err := c.Match(ctx, &service.MatchReq{
		Package:      *matchPackage,
		Repository:   *matchRepo,
		CountOnly:    *doCountOnly,
		IncludeFiles: true,
		Limit:        *rowLimit,
	}, func(row *graph.Row) error {
		if *doKeysOnly {
			fmt.Println(row.ImportPath)
		} else {
			enc.Encode(row)
		}
		return nil
	})
	if err != nil {
		log.Printf("Match failed: %v", err)
	} else if *doCountOnly {
		fmt.Println(nr)
	} else if nr == 0 {
		log.Printf("No packages matching %q", *matchPackage)
	}
}
