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

// Program revdeps lists the reverse dependencies of a package.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/tools"
)

var storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	for _, pkg := range flag.Args() {
		if err := g.Importers(ctx, pkg, func(ipath string) {
			fmt.Println(ipath)
		}); err != nil {
			log.Fatalf("Importers failed: %v", err)
		}
	}
}
