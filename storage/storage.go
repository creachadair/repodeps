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

// Package storage defines a persistent storage interface.
package storage

import (
	"context"
	"errors"

	"github.com/creachadair/ffs/blob"
	"google.golang.org/protobuf/proto"
)

var (
	// ErrKeyNotFound is returned by Load when the specified key is not found.
	ErrKeyNotFound = errors.New("key not found")

	// ErrStopScan is returned by the callback to Scan to terminate a scan.
	ErrStopScan = errors.New("stop scanning")
)

// Interface represents the interface to persistent storage.
type Interface interface {
	// Load reads the data for the specified key and unmarshals it into val.
	// If key is not present, Load must return storage.ErrKeyNotFound.
	Load(ctx context.Context, key string, val proto.Message) error

	// Store marshals the data from value and stores it under key.
	Store(ctx context.Context, key string, val proto.Message) error

	// Scan calls f with each key lexicographically greater than or equal to
	// start.  If f reports an error scanning stops; if the error is ErrStopScan
	// then Scan return nil, otherwise Scan returns the error from f.
	Scan(ctx context.Context, start string, f func(string) error) error

	// Delete removes the specified key from the database.
	Delete(ctx context.Context, key string) error
}

// NewBlob constructs a storage implementation around a blob.Store.
func NewBlob(bs blob.Store) BlobStore { return BlobStore{bs: bs} }

// BlobStore implements the package Storage interfaces.
type BlobStore struct {
	bs blob.Store
}

// Load implements part of graph.Storage and poll.Storage
func (s BlobStore) Load(ctx context.Context, key string, val proto.Message) error {
	bits, err := s.bs.Get(ctx, key)
	if errors.Is(err, blob.ErrKeyNotFound) {
		return ErrKeyNotFound
	} else if err != nil {
		return err
	}
	return proto.Unmarshal(bits, val)
}

// Store implements part of graph.Storage and poll.Storage.
func (s BlobStore) Store(ctx context.Context, key string, val proto.Message) error {
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

// Scan implements part of graph.Storage and poll.Storage.
func (s BlobStore) Scan(ctx context.Context, start string, f func(string) error) error {
	return s.bs.List(ctx, start, func(key string) error {
		if err := f(key); err == ErrStopScan {
			return blob.ErrStopListing
		} else if err != nil {
			return err
		}
		return nil
	})
}

// Delete implements part of poll.Storage.
func (s BlobStore) Delete(ctx context.Context, key string) error {
	return s.bs.Delete(ctx, key)
}
