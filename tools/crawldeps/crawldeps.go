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

// Program crawldeps polls for repositories that need to be updated, and
// recomputes ranking data after updates are performed.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/repodeps/client"
	"github.com/creachadair/repodeps/service"
)

var (
	address      = flag.String("address", os.Getenv("DEPSERVER_ADDR"), "Service address")
	pollInterval = flag.Duration("interval", 720*time.Second, "Poll interval")
	sampleRate   = flag.Float64("sample-rate", 0.1, "Sampling rate for updates")
)

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage: %[1]s [options]

Poll a dependency database for repositories that have been updated, and 
update the contents and rankings for any that have changed.

Options:
`, filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
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

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		log.Printf("Received signal: %v; shutting down in 2 seconds", <-sig)
		signal.Stop(sig)
		time.Sleep(2 * time.Second) // give in-flight requests time to cancel
		cancel()
	}()
	ticker := time.NewTicker(*pollInterval)

	// Handle push logs from the server.
	go c.Receive(ctx, func(req *jrpc2.Request) {
		var params json.RawMessage
		req.UnmarshalParams(&params)
		log.Printf("[from server]: %s %s", req.Method(), string(params))
	})

	for {
		func() { // simplify error recovery; return falls through to the update.
			scanRsp, err := c.Scan(ctx, &service.ScanReq{
				SampleRate: *sampleRate,
				LogUpdates: true,
			})
			if err != nil {
				log.Printf("Scan failed: %v", err)
				return
			}
			log.Printf("Scan complete [%v elapsed]: scanned=%d, dups=%d, samples=%d, updates=%d, pkgs=%d",
				scanRsp.Elapsed, scanRsp.NumScanned, scanRsp.NumDups, scanRsp.NumSamples,
				scanRsp.NumUpdates, scanRsp.NumPackages)

			rankRsp, err := c.Rank(ctx, &service.RankReq{
				Update: true,
			})
			if err != nil {
				log.Printf("Rank failed: %v", err)
				return
			}
			log.Printf("Rank complete [%v elapsed]: rows=%d, ranks=%d",
				rankRsp.Elapsed, rankRsp.NumRows, rankRsp.NumRanks)
		}()
		log.Printf("Waiting for next update...")
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
		}
	}
}
