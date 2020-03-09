// Copyright 2020 Michael J. Fromberger. All Rights Reserved.
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

// Program updatedeps adds the specified repositories to a depserver.
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
	address = flag.String("address", os.Getenv("DEPSERVER_ADDR"), "Service address")

	base = service.UpdateReq{Options: new(deps.Options)}
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options] <args>

Update the specified repositories in a dependency server. Each non-flag argument
is the URL of a repository to update.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.StringVar(&base.Reference, "ref", "", "The reference name to update")
	flag.StringVar(&base.Prefix, "prefix", "", "Prefix to attribute to packages")
	flag.BoolVar(&base.CheckOnly, "check", false, "Only check repository state; do not update")
	flag.BoolVar(&base.Reset, "reset", false, "Remove existing packages before update")
	flag.BoolVar(&base.Force, "force", false, "Force update even if up-to-date")
	flag.BoolVar(&base.Options.StandardLibrary, "stdlib", false, "Treat packages as standard libraries")
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	c, err := client.Dial(ctx, *address)
	if err != nil {
		log.Fatalf("Dialing service: %v", err)
	}
	defer c.Close()
	log.Printf("Connected to %q", *address)
	c.SetToken(os.Getenv("DEPSERVER_WRITE_TOKEN"))

	out := json.NewEncoder(os.Stdout)
	for _, url := range flag.Args() {
		req := base
		req.Repository = url
		rsp, err := c.Update(ctx, &req)
		if err != nil {
			log.Printf("ERROR: Update(%q): %v", url, err)
			continue
		}
		out.Encode(rsp)
	}
	// TODO: Exit status.
}
