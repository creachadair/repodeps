// Package storage defines implementations of the graph.Storage interface.
package storage

import (
	"context"

	"github.com/creachadair/ffs/blob"
	"github.com/creachadair/repodeps/graph"
	"github.com/golang/protobuf/proto"
)

// NewBlob constructs a graph.Storage implementation around a blob.Store.
func NewBlob(bs blob.Store) graph.Storage { return storage{bs: bs} }

type storage struct {
	bs blob.Store
}

// Load implements part of the graph.Storage interface.
func (s storage) Load(ctx context.Context, key string, val proto.Message) error {
	bits, err := s.bs.Get(ctx, key)
	if err != nil {
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
