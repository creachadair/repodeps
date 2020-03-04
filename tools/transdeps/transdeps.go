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

// Program transdeps lists the transitive closure of forward dependencies of a
// set of packages.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
)

var (
	address = flag.String("address", os.Getenv("DEPSERVER_ADDR"), "Service address")

	noStdLib = flag.Bool("no-stdlib", false, "Filter out standard library packages")
)

func main() {
	flag.Parse()

	ctx := context.Background()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	seen := stringset.New()
	dead := stringset.New()
	todo := stringset.New(flag.Args()...)
	for len(todo) != 0 {
		for pkg := range todo {
			todo.Discard(pkg)
			if seen.Contains(pkg) {
				continue
			}
			c.Match(ctx, &service.MatchReq{Package: pkg}, func(row *graph.Row) error {
				if seen.Contains(row.ImportPath) {
					return nil
				}
				seen.Add(row.ImportPath)
				if *noStdLib && row.Type == graph.Row_STDLIB {
					dead.Add(row.ImportPath)
				}
				for _, dep := range row.Directs {
					todo.Add(dep)
				}
				return nil
			})
		}
		todo.Remove(seen)
	}
	seen.Remove(dead)
	for _, pkg := range seen.Elements() {
		fmt.Println(pkg)
	}
}
