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

// Package service defines a service that maintains the state of a dependency
// graph. It is compatible with the github.com/creachadair/jrpc2 package, but
// can also be used independently.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/deps"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
)

// Options control the behaviour of a Server.
type Options struct {
	RepoDB  string // path of repository state database (required)
	GraphDB string // path of graph database (required)
	WorkDir string // working directory for update clones

	// The minimum polling interval for repository scans.
	MinPollInterval time.Duration

	// The maximum number of times a repository update may fail before that
	// repository is removed from the database.
	ErrorLimit int

	// Default sampling rate for scans (0..1); zero means 1.0.
	SampleRate float64

	// Default scale factor for ranking; zero means 4.
	RankScale int

	// Default damping factor for ranking; zero means 0.85.
	RankDamping float64

	// Default iteration count for ranking; zero means 10.
	RankIterations int

	// The maximum number of concurrent workers that may be processing updates.
	// A value less that or equal to zero means 1.
	Concurrency int

	// If set, this callback is invoked to deliver streaming logs from scan
	// operations. The server ensures that at most one goroutine is active in
	// this callback at once.
	StreamLog func(ctx context.Context, key string, value interface{}) error

	// The default page size for paginated responses (0 means 100).
	DefaultPageSize int

	// Open read-only, disallow updates.
	ReadOnly bool

	// Default package loader options.
	deps.Options
}

func (o Options) merge(opts *deps.Options) *deps.Options {
	out := o.Options
	if opts != nil {
		out.HashSourceFiles = out.HashSourceFiles || opts.HashSourceFiles
		out.UseImportComments = out.UseImportComments || opts.UseImportComments
		out.TrimRepoPrefix = out.TrimRepoPrefix || opts.TrimRepoPrefix
		out.StandardLibrary = out.StandardLibrary || opts.StandardLibrary
	}
	return &out
}

// KeyNotFound is the error code returned when a requested key is not found in
// the database.
var KeyNotFound = code.Register(404, "key not found")

// New constructs a new Server from the specified options.  As long as the
// server is open, it holds a lock on the databases assigned to it.
// The caller must call Close when the server is no longer in use.
func New(opts Options) (*Server, error) {
	if opts.RepoDB == "" {
		return nil, errors.New("no repository database")
	}
	if opts.GraphDB == "" {
		return nil, errors.New("no graph database")
	}
	if opts.SampleRate == 0 {
		opts.SampleRate = 1
	}
	if opts.RankScale <= 0 {
		opts.RankScale = 4
	}
	if opts.RankDamping == 0 {
		opts.RankDamping = 0.85
	}
	if opts.RankIterations <= 0 {
		opts.RankIterations = 10
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	if opts.DefaultPageSize <= 0 {
		opts.DefaultPageSize = 100
	}
	u := &Server{opts: opts}
	if f := opts.StreamLog; f != nil {
		mu := new(sync.Mutex)
		u.log = func(ctx context.Context, key string, arg interface{}) error {
			mu.Lock()
			defer mu.Unlock()
			return f(ctx, key, arg)
		}
	} else {
		u.log = jrpc2.ServerPush
	}
	openBadger := badgerstore.NewPath
	if opts.ReadOnly {
		openBadger = badgerstore.NewPathReadOnly
	}

	if s, err := openBadger(opts.RepoDB); err == nil {
		u.repoDB = poll.NewDB(storage.NewBlob(s))
		u.repoC = s
	} else {
		return nil, fmt.Errorf("opening repository database: %v", err)
	}
	if s, err := openBadger(opts.GraphDB); err == nil {
		u.graph = graph.New(storage.NewBlob(s))
		u.graphC = s
	} else {
		u.repoC.Close()
		return nil, fmt.Errorf("opening graph database: %v", err)
	}

	return u, nil
}

// A Server manages reads and updates to a database of dependencies.
type Server struct {
	repoDB *poll.DB
	repoC  io.Closer
	graph  *graph.Graph
	graphC io.Closer

	scanning int32
	opts     Options

	log func(context.Context, string, interface{}) error
}

// Close shuts down the server and closes its underlying data stores.
func (u *Server) Close() error {
	gerr := u.graphC.Close()
	rerr := u.repoC.Close()
	if gerr != nil {
		return gerr
	}
	return rerr
}

func (u *Server) pushLog(ctx context.Context, sel bool, key string, arg interface{}) {
	if !sel {
		return
	}
	switch t := arg.(type) {
	case *jrpc2.Error:
		// nothing special
	case error:
		arg = struct {
			E string `json:"message"`
		}{t.Error()}
	}
	u.log(ctx, key, arg)
}

// A StringList is a slice of strings that can be decoded from JSON as either
// an array or a single string.
type StringList []string

// UnmarshalJSON decodes a StringList from JSON, accepting either a string
// value (corresponding to a single-element slice) or an array of strings.
func (s *StringList) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		*s = nil
		return nil
	} else if data[0] == '"' {
		*s = []string{""}
		return json.Unmarshal(data, &(*s)[0])
	}
	return json.Unmarshal(data, (*[]string)(s))
}
