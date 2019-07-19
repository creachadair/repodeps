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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/service"
)

var (
	address    = flag.String("address", os.Getenv("REPODEPS_ADDR"), "Service address")
	countOnly  = flag.Bool("count", false, "Count the number of matching dependencies")
	filterSame = flag.Bool("filter-same-repo", false, "Exclude dependencies from the same repository")
	filterDom  = flag.Bool("domain-only", false, "Exclude local and intrinsic imports")
	limit      = flag.Int("limit", 0, "Return at most this many results")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s <package>

Print the import paths of packages that depend directly on each named package.
If a package ends with "/...", it matches any package with the given prefix.
Each output is a JSON text:

   {"target": target-package, "source": source-package}

where source-package is the dependent (importing) package and target-package is
the dependency (imported) package.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		log.Fatal("You must provide at least one package or prefix to match")
	}

	ctx := context.Background()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	enc := json.NewEncoder(os.Stdout)
	nr, err := c.Reverse(ctx, &service.ReverseReq{
		Package:        flag.Args(),
		CountOnly:      *countOnly,
		FilterSameRepo: *filterSame,
		Limit:          *limit,
	}, func(dep *service.ReverseDep) error {
		if _, ok := deps.HasDomain(dep.Source); ok || !*filterDom {
			enc.Encode(dep)
		}
		return nil
	})
	if err != nil {
		log.Printf("Reverse failed: %v", err)
	} else if *countOnly {
		fmt.Println(nr)
	} else if nr == 0 {
		log.Printf("No reverse dependencies matching %q", flag.Arg(0))
	}
}
