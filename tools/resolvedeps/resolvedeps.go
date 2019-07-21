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

// Program resolvedeps resolves package repositories.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"

	"bitbucket.org/creachadair/stringset"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/service"
	"github.com/creachadair/repodeps/tools"
	"github.com/creachadair/taskgroup"
)

var (
	address = flag.String("address", os.Getenv("REPODEPS_ADDR"), "Service address")

	doReadStdin = flag.Bool("stdin", false, "Read package names from stdin")
	numWorkers  = flag.Int("workers", 16, "Numcer of concurrent workers")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()

	var mu sync.RWMutex
	pfxs := stringset.New()
	seen := func(pkg string) bool {
		mu.RLock()
		defer mu.RUnlock()
		for pfx := range pfxs {
			if strings.HasPrefix(pkg, pfx) {
				return true
			}
		}
		return false
	}
	addPfx := func(pfx string) {
		mu.Lock()
		defer mu.Unlock()
		pfxs.Add(pfx)
	}
	repo := stringset.New()
	addRepo := func(url string) {
		mu.Lock()
		defer mu.Unlock()
		repo.Add(url)
	}
	g, run := taskgroup.New(taskgroup.Trigger(cancel)).Limit(*numWorkers)

	for pkg := range tools.Inputs(*doReadStdin) {
		pkg := pkg
		run(func() error {
			if seen(pkg) {
				return nil
			}
			rsp, err := c.Resolve(ctx, pkg)
			if err != nil {
				log.Printf("Resolving %q: %v", pkg, err)
				return nil
			}
			addPfx(rsp.Prefix)
			_, err = c.RepoStatus(ctx, rsp.Repository)
			if code.FromError(err) == service.KeyNotFound {
				addRepo(rsp.Repository)
			} else if err != nil {
				log.Printf("Repo status %q: %v", rsp.Repository, err)
			}
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
	for _, url := range repo.Elements() {
		fmt.Println(url)
	}
}
