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
	"path/filepath"
	"regexp"
	"strings"

	"github.com/creachadair/repodeps/tools"
)

var (
	storePath   = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	doFilterDom = flag.Bool("domain-only", false, "Print only import paths that begin with a domain")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s <package>...

Print the import paths of packages that depend directly on each named package.
If a package ends with "/...", it matches any package with the given prefix.
Each output line has the form:

   target-package <TAB> source-package <LF>

where source-package is the dependent package and target-package is the
imported package.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraph(*storePath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	ctx := context.Background()
	if err := g.MatchImporters(ctx, newMatcher(flag.Args()), func(tpath, ipath string) {
		if *doFilterDom {
			if _, ok := tools.HasDomain(tpath); !ok {
				return
			}
			if _, ok := tools.HasDomain(ipath); !ok {
				return
			}
		}
		fmt.Print(tpath, "\t", ipath, "\n")
	}); err != nil {
		log.Fatalf("Importers failed: %v", err)
	}
}

func newMatcher(args []string) func(string) bool {
	var ps []string
	for _, arg := range args {
		if t := strings.TrimSuffix(arg, "/..."); t != arg {
			ps = append(ps, regexp.QuoteMeta(t))
		} else {
			ps = append(ps, regexp.QuoteMeta(arg)+"$")
		}
	}
	re := regexp.MustCompile("^(?:" + strings.Join(ps, "|") + ")")
	return re.MatchString
}
