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

// Program depserver implements a JSON-RPC service giving access to the
// contents of a dependency graph.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/metrics"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/repodeps/service"
)

var (
	opts service.Options

	serviceAddr = flag.String("address", "", "Service address (required)")
	repoDB      = os.Getenv("DEPSERVER_REPO_DB")
	graphDB     = os.Getenv("DEPSERVER_GRAPH_DB")
	workDir     = os.Getenv("DEPSERVER_WORK_DIR")
	writeToken  = os.Getenv("DEPSERVER_WRITE_TOKEN")
)

func init() {
	flag.StringVar(&opts.RepoDB, "repo-db", repoDB, "Repository database (required; $DEPSERVER_REPO_DB)")
	flag.StringVar(&opts.GraphDB, "graph-db", graphDB, "Graph database (required; $DEPSERVER_GRAPH_DB)")
	flag.StringVar(&opts.WorkDir, "workdir", workDir, "Working directory for updates ($DEPSERVER_WORK_DIR)")
	flag.DurationVar(&opts.MinPollInterval, "interval", 1*time.Hour, "Minimum scan interval")
	flag.IntVar(&opts.ErrorLimit, "error-limit", 10, "Maximum repository update failures")
	flag.Float64Var(&opts.SampleRate, "sample-rate", 1, "Sample fraction of eligible updates (0..1)")
	flag.IntVar(&opts.RankScale, "rank-scale", 4, "Scale rank values to this many significant figures")
	flag.Float64Var(&opts.RankDamping, "rank-damping", 0.85, "Damping factor for ranking (0..1)")
	flag.IntVar(&opts.RankIterations, "rank-iter", 10, "Default iteration count for ranking")
	flag.IntVar(&opts.Concurrency, "concurrency", 16, "Maximum concurrent updates")
	flag.IntVar(&opts.DefaultPageSize, "page-size", 100, "Default result page size")
	flag.BoolVar(&opts.ReadOnly, "read-only", false, "Open database read-only, disallowing updates")

	flag.BoolVar(&opts.Options.HashSourceFiles, "hash-source-files", true,
		"Record source file digests")
	flag.BoolVar(&opts.Options.UseImportComments, "use-import-comments", true,
		"Parse import comments to name packages")
}

func main() {
	flag.Parse()
	if *serviceAddr == "" {
		log.Fatal("You must provide a non-empty service -address")
	}
	lst, err := net.Listen(jrpc2.Network(*serviceAddr))
	if err != nil {
		log.Fatalf("Listen %q: %v", *serviceAddr, err)
	}
	log.Printf(`Updater service starting:
- Listening at:        %s
- Repository database: %s
- Graph database:      %s
`, *serviceAddr, opts.RepoDB, opts.GraphDB)

	sig := make(chan os.Signal, 2)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		log.Printf("Received signal: %v; shutting down", <-sig)
		lst.Close()
		signal.Stop(sig)
	}()

	u, err := service.New(opts)
	if err != nil {
		log.Fatalf("Creating updater: %v", err)
	}
	m := metrics.New()
	if writeToken != "" {
		m.SetLabel("token", writeToken)
	}
	if opts.ReadOnly {
		m.SetLabel("mode", "read-only")
	} else {
		m.SetLabel("mode", "read-write")
	}
	acc := server.NetAccepter(lst, channel.Line)
	if err := server.Loop(acc, server.Static(u.Methods()), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			AllowPush:     true,
			DecodeContext: jctx.Decode,
			Metrics:       m,
			CheckRequest:  checkAccess,
		},
	}); err != nil {
		log.Printf("Server loop failed: %v", err)
	}
	log.Printf("Server loop ended, shutting down")
	if err := u.Close(); err != nil {
		log.Fatalf("Closing updater: %v", err)
	}
}

func checkAccess(ctx context.Context, req *jrpc2.Request) error {
	switch req.Method() {
	case "Rank", "Remove", "Scan", "Update":
		if writeToken == "" {
			return nil
		}
	default:
		return nil // this method does not write
	}
	var tok string
	if err := jctx.UnmarshalMetadata(ctx, &tok); err != nil {
		return jrpc2.Errorf(400, "write token not present")
	} else if tok != writeToken {
		return jrpc2.Errorf(401, "method not allowed")
	}
	return nil
}
