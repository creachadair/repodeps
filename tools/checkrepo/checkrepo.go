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

// Program checkrepo checks the current status of one or more repositories
// against a database of known latest digests.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"
	"sync"

	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

var (
	storePath   = flag.String("store", "", "Storage path (required)")
	doReadStdin = flag.Bool("stdin", false, "Read repo URLs from stdin")
	concurrency = flag.Int("concurrency", 16, "Number of concurrent workers")
)

func main() {
	flag.Parse()

	db, c, err := tools.OpenPollDB(*storePath, tools.ReadWrite)
	if err != nil {
		log.Fatalf("Opening poll database: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(*concurrency)
	var omu sync.Mutex
	enc := json.NewEncoder(os.Stdout)
	for url := range tools.Inputs(*doReadStdin) {
		run(func() error {
			res, err := db.Check(ctx, url)
			if err != nil {
				log.Printf("[skipped] checking %q: %v", url, err)
			} else {
				omu.Lock()
				enc.Encode(result{
					Need: res.NeedsUpdate(),
					Repo: res.URL,
					Name: res.Name,
					Hex:  res.Digest,
				})
				omu.Unlock()
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatalf("Processing failed: %v", err)
	}
	if err := c.Close(); err != nil {
		log.Fatalf("Closing storage: %v", err)
	}
}

type result struct {
	Need bool   `json:"needsUpdate"`
	Repo string `json:"repository"`
	Name string `json:"name"`
	Hex  string `json:"hexDigest"`
}
