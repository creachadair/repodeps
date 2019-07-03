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

// Program repostat prints the current status of one or more repositories
// against a database of known latest digests.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/creachadair/repodeps/tools"
	"github.com/golang/protobuf/jsonpb"
)

var (
	pollDBPath  = flag.String("polldb", os.Getenv("REPODEPS_POLLDB"), "Poll database path (required)")
	doReadStdin = flag.Bool("stdin", false, "Read repo URLs from stdin")
)

func main() {
	flag.Parse()

	db, c, err := tools.OpenPollDB(*pollDBPath, tools.ReadOnly)
	if err != nil {
		log.Fatalf("Opening poll database: %v", err)
	}
	defer c.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	urls := tools.Inputs(*doReadStdin)
	var enc jsonpb.Marshaler

	for url := range urls {
		url := tools.FixRepoURL(url)
		stat, err := db.Status(ctx, url)
		if err != nil {
			log.Printf("[skipped] status for %q: %v", url, err)
			continue
		}
		enc.Marshal(os.Stdout, stat)
		fmt.Println()
	}
}
