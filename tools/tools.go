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

// Package tools implements shared code for command-line tools.
package tools

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/creachadair/badgerstore"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/storage"
)

var storePath = flag.String("store", os.Getenv("REPODEPS_DB"), "Storage path (required)")

// OpenGraph opens the graph indicated by the -store flag.
// The caller must ensure the closer is closed.
func OpenGraph() (*graph.Graph, io.Closer, error) {
	if *storePath == "" {
		return nil, nil, errors.New("no -store path was provided")
	}
	s, err := badgerstore.NewPath(*storePath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening storage: %v", err)
	}
	return graph.New(storage.NewBlob(s)), s, nil
}
