package main

import (
	"flag"
	"log"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/handler"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/jrpc2/server"
	"github.com/creachadair/repodeps/updater"
)

var (
	opts updater.Options

	serviceAddr = flag.String("address", "", "Service address (required)")
	repoDB      = os.Getenv("REPODEPS_POLLDB")
	graphDB     = os.Getenv("REPODEPS_DB")
)

func init() {
	flag.StringVar(&opts.RepoDB, "repo-db", repoDB, "Repository database (required; $REPODEPS_POLLDB)")
	flag.StringVar(&opts.GraphDB, "graph-db", graphDB, "Graph database (required; $REPODEPS_DB)")
	flag.StringVar(&opts.WorkDir, "workdir", "", "Working directory for updates")
	flag.DurationVar(&opts.MinPollInterval, "interval", 1*time.Hour, "Minimum scan interval")
	flag.IntVar(&opts.ErrorLimit, "error-limit", 10, "Maximum repository update failures")
	flag.Float64Var(&opts.SampleRate, "sample-rate", 1, "Sample fraction of eligible updates (0..1)")
	flag.IntVar(&opts.Concurrency, "concurrency", 16, "Maximum concurrent updates")

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
	lst, err := net.Listen(jrpc2.Network(*serviceAddr), *serviceAddr)
	if err != nil {
		log.Fatalf("Listen %q: %v", *serviceAddr, err)
	}
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		log.Printf("Received signal: %v; shutting down", <-sig)
		lst.Close()
		signal.Stop(sig)
	}()

	u, err := updater.New(opts)
	if err != nil {
		log.Fatalf("Creating updater: %v", err)
	}
	if err := server.Loop(lst, handler.NewService(u), &server.LoopOptions{
		ServerOptions: &jrpc2.ServerOptions{
			AllowPush:     true,
			DecodeContext: jctx.Decode,
		},
	}); err != nil {
		log.Printf("Server loop failed: %v", err)
	}
	if err := u.Close(); err != nil {
		log.Fatalf("Closing updater: %v", err)
	}
}
