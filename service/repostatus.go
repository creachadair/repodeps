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

package service

import (
	"context"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/poll"
	"github.com/creachadair/repodeps/storage"
)

// RepoStatus reports the current status of the specified repository.
func (u *Server) RepoStatus(ctx context.Context, req *RepoStatusReq) (*RepoStatusRsp, error) {
	if req.Repository == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "empty repository URL")
	}
	stats, err := u.repoDB.Tags(ctx, poll.FixRepoURL(req.Repository))
	if err == storage.ErrKeyNotFound {
		return nil, jrpc2.Errorf(KeyNotFound, "repo %q not found", req.Repository)
	} else if err != nil {
		return nil, err
	}
	return &RepoStatusRsp{Status: stats}, nil
}

// RepoStatusReq is the request parameter to the RepoStatus method.
type RepoStatusReq struct {
	Repository string `json:"repository"`
}

// RepoStatusRsp is the response message from a successful RepoStatus call.
type RepoStatusRsp struct {
	Status []*poll.Status `json:"status,omitempty"`
}
