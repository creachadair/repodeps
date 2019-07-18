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

	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
)

var address = flag.String("address", os.Getenv("REPODEPS_ADDR"), "Service address")

func main() {
	flag.Parse()

	ctx := context.Background()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	enc := json.NewEncoder(os.Stdout)
	for _, ipath := range flag.Args() {
		nr, err := c.Match(ctx, &service.MatchReq{
			Package:      ipath,
			IncludeFiles: true,
		}, func(row *graph.Row) error {
			enc.Encode(row)
			return nil
		})
		if err != nil {
			log.Printf("Reading %q: %v", ipath, err)
		} else if nr == 0 {
			log.Printf("Package %q not found", ipath)
		}
	}
}
