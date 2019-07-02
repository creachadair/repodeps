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

// Program exportdeps export graph into a CSV in adjacency list format
//  ready to load in Gephi.
// See https://gephi.org/users/supported-graph-formats/csv-format/
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/tools"
)

const sep = ";"

type dict struct {
	pathToID map[string]int
	lastID   int
}

var (
	storePath  = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")
	useIDs     = flag.Bool("ids", false, "Use int IDs instead for import paths")
	skipStdlib = flag.Bool("skipStdlib", false, "Skip stdlib packages (without .)")
	skipNodeps = flag.Bool("skipNodeps", false, "Skip packages without any dependencies")
	pathToID   = dict{make(map[string]int), 0}
)

func (d *dict) lookupMaybeAdd(path string) string {
	var id int
	var ok bool
	if id, ok = d.pathToID[path]; !ok {
		id = d.lastID
		d.pathToID[path] = id
		d.lastID++
	}
	return strconv.FormatInt(int64(id), 10)
}

func main() {
	flag.Parse()
	g, c, err := tools.OpenGraphReadOnly(*storePath)
	if err != nil {
		log.Fatalf("Opening graph: %v", err)
	}
	defer c.Close()

	if err := g.Scan(context.Background(), "", func(row *graph.Row) error {
		if *skipStdlib && !strings.Contains(row.ImportPath, ".") {
			return nil
		}

		fromNode := row.ImportPath
		if *useIDs {
			fromNode = pathToID.lookupMaybeAdd(fromNode)
		}

		var toNodes []string
		for _, imprt := range row.Directs {
			if *skipStdlib && !strings.Contains(imprt, ".") {
				continue
			}
			if *useIDs {
				imprt = pathToID.lookupMaybeAdd(imprt)
			}
			toNodes = append(toNodes, imprt)
		}
		if *skipNodeps && len(toNodes) == 0 {
			return nil
		}

		fmt.Printf("%s%s%s\n", fromNode, sep, strings.Join(toNodes, sep))
		return nil
	}); err != nil {
		log.Fatalf("Scan failed: %v", err)
	}

	if *useIDs {
		log.Printf("Total uniq nodes: %d", len(pathToID.pathToID))

		const vocabFile = "ids-vocab.txt"
		log.Printf("Saving ID vocabulary to %s", vocabFile)
		var bb bytes.Buffer
		for k, v := range pathToID.pathToID {
			bb.WriteString(fmt.Sprintf("%d %s\n", v, k))
		}
		err := ioutil.WriteFile(vocabFile, bb.Bytes(), 0644)
		if err != nil {
			log.Fatalf("Saving the ID vocab: %v", err)
		}
	}
}
