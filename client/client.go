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

// Package client defines a client for the dependency service defined by the
// service package.
package client

import (
	"context"
	"fmt"
	"net"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/jctx"
	"github.com/creachadair/repodeps/graph"
	"github.com/creachadair/repodeps/service"
)

// A Client implements a JSON-RPC client to the dependency service exported by
// the service package.
type Client struct {
	cli    *jrpc2.Client
	notes  chan *jrpc2.Request
	token  string
	cancel func()
}

// Dial establishes a connection to the dependency service at addr.  If ctx
// contains a deadline, that timeout governs the dial operation and not the
// operation of the resulting client.
func Dial(ctx context.Context, addr string) (*Client, error) {
	var d net.Dialer
	net, addr := jrpc2.Network(addr)
	conn, err := d.DialContext(ctx, net, addr)
	if err != nil {
		return nil, err
	}

	// Get a cancellable but detached context to control the notifier.
	ctx, cancel := context.WithCancel(context.Background())
	ch := channel.Line(conn, conn)
	notes := make(chan *jrpc2.Request)
	return &Client{
		cli: jrpc2.NewClient(ch, &jrpc2.ClientOptions{
			EncodeContext: jctx.Encode,
			OnNotify: func(req *jrpc2.Request) {
				select {
				case notes <- req:
					// deliver a notification if possible
				case <-ctx.Done():
					// the client is shutting down
					close(notes)
				}
			},
		}),
		notes:  notes,
		cancel: cancel,
	}, nil
}

// SetToken sets a write token that will be passed to the server during
// requests that modify data.
func (c *Client) SetToken(token string) { c.token = token }

// Receive calls f with each server notification received by c as long as ctx
// is active. Calls to f are synchronous with request processing.
func (c *Client) Receive(ctx context.Context, f func(*jrpc2.Request)) {
	for {
		select {
		case <-ctx.Done():
			return // the request is done; don't accept any more pushes
		case req := <-c.notes:
			f(req)
		}
	}
}

// Close shuts down the client, terminating any pending calls.
func (c *Client) Close() error { c.cancel(); return c.cli.Close() }

// Match calls the eponymous method of the service and delivers a response to f
// for each result row. If f reports an error, pagination stops and that error
// is reported to the caller of Match. The total number of matching rows is
// returned.
func (c *Client) Match(ctx context.Context, req *service.MatchReq, f func(*graph.Row) error) (int, error) {
	cp := *req
	lim := cp.Limit
	nr := 0
	for {
		var rsp service.MatchRsp
		if err := c.cli.CallResult(ctx, "Match", &cp, &rsp); err != nil {
			return nr, err
		} else if req.CountOnly {
			return rsp.NumRows, nil
		}
		for _, row := range rsp.Rows {
			err := f(row)
			nr++
			if err != nil {
				return nr, err
			} else if lim > 0 && nr == lim {
				return nr, nil
			}
		}
		if rsp.NextPage == nil {
			return nr, nil
		}
		cp.PageKey = rsp.NextPage
	}
}

// Resolve calls the eponymous method of the service.
func (c *Client) Resolve(ctx context.Context, pkg string) (*service.ResolveRsp, error) {
	var rsp service.ResolveRsp
	if err := c.cli.CallResult(ctx, "Resolve", &service.ResolveReq{Package: pkg}, &rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

// Reverse calls the eponymous method of the service and delivers a result to f
// for each reverse dependency found. If f reports an error, pagination stops
// and that error is reported to the caller of Reverse. The total number of
// matching rows is returned.
func (c *Client) Reverse(ctx context.Context, req *service.ReverseReq, f func(*service.ReverseDep) error) (int, error) {
	cp := *req
	lim := cp.Limit
	nr := 0
	for {
		var rsp service.ReverseRsp
		if err := c.cli.CallResult(ctx, "Reverse", &cp, &rsp); err != nil {
			return nr, err
		} else if req.CountOnly {
			return rsp.NumImports, nil
		}
		for _, imp := range rsp.Imports {
			err := f(imp)
			nr++
			if err != nil {
				return nr, err
			} else if lim > 0 && nr == lim {
				return nr, nil
			}
		}
		if rsp.NextPage == nil {
			return nr, nil
		}
		cp.PageKey = rsp.NextPage
	}
}

// RepoStatus calls the eponymous method of the service.
func (c *Client) RepoStatus(ctx context.Context, repo string) (*service.RepoStatusRsp, error) {
	var rsp service.RepoStatusRsp
	if err := c.cli.CallResult(ctx, "RepoStatus", &service.RepoStatusReq{
		Repository: repo,
	}, &rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

// Scan calls the eponymous method of the service. If the server requires a
// write token, the caller must provide one via SetToken.
func (c *Client) Scan(ctx context.Context, req *service.ScanReq) (*service.ScanRsp, error) {
	ctx, err := jctx.WithMetadata(ctx, c.token)
	if err != nil {
		return nil, fmt.Errorf("write token: %v", err)
	}
	var rsp service.ScanRsp
	if err := c.cli.CallResult(ctx, "Scan", req, &rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

// Rank calls the eponymous method of the service. If the server requires a
// write token, the caller must provide one via SetToken.
func (c *Client) Rank(ctx context.Context, req *service.RankReq) (*service.RankRsp, error) {
	ctx, err := jctx.WithMetadata(ctx, c.token)
	if err != nil {
		return nil, fmt.Errorf("write token: %v", err)
	}
	var rsp service.RankRsp
	if err := c.cli.CallResult(ctx, "Rank", req, &rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

// Update calls the eponymous method of the service. If the server requires a
// write token, the caller must provide one via SetToken.
func (c *Client) Update(ctx context.Context, req *service.UpdateReq) (*service.UpdateRsp, error) {
	ctx, err := jctx.WithMetadata(ctx, c.token)
	if err != nil {
		return nil, fmt.Errorf("write token: %v", err)
	}
	var rsp service.UpdateRsp
	if err := c.cli.CallResult(ctx, "Update", req, &rsp); err != nil {
		return nil, err
	}
	return &rsp, nil
}

// TODO: Implement the rest of the methods.
