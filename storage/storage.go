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

// Package storage defines implementations of the graph.Storage interface.
package storage

import (
	"context"
	"strings"

	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/repodeps/graph"
	"github.com/golang/protobuf/proto"
	"golang.org/x/xerrors"
)

// NewBlob constructs a graph.Storage implementation around a blob.Store.
func NewBlob(bs blob.Store) graph.Storage { return storage{bs: bs} }

type storage struct {
	bs blob.Store
}

// Load implements part of the graph.Storage interface.
func (s storage) Load(ctx context.Context, key string, val proto.Message) error {
	bits, err := s.bs.Get(ctx, key)
	if xerrors.Is(err, blob.ErrKeyNotFound) {
		return graph.ErrKeyNotFound
	} else if err != nil {
		return err
	}
	return proto.Unmarshal(bits, val)
}

// Store implements part of the graph.Storage interface.
func (s storage) Store(ctx context.Context, key string, val proto.Message) error {
	bits, err := proto.Marshal(val)
	if err != nil {
		return err
	}
	return s.bs.Put(ctx, blob.PutOptions{
		Key:     key,
		Data:    bits,
		Replace: true,
	})
}

// Scan implements part of the graph.Storage interface.
func (s storage) Scan(ctx context.Context, prefix string, f func(string) error) error {
	return s.bs.List(ctx, prefix, func(key string) error {
		if !strings.HasPrefix(key, prefix) {
			return blob.ErrStopListing
		} else if err := f(key); err != nil {
			return err
		}
		return nil
	})
}
