package service

import (
	"context"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/code"
	"github.com/creachadair/repodeps/poll"
)

// RepoStatus reports the current status of the specified repository.
func (u *Server) RepoStatus(ctx context.Context, req *RepoStatusReq) (*RepoStatusRsp, error) {
	if req.Repository == "" {
		return nil, jrpc2.Errorf(code.InvalidParams, "empty repository URL")
	}
	stat, err := u.repoDB.Status(ctx, poll.FixRepoURL(req.Repository))
	if err != nil {
		return nil, err
	}
	return &RepoStatusRsp{Status: stat}, nil
}

// RepoStatusReq is the request parameter to the RepoStatus method.
type RepoStatusReq struct {
	Repository string `json:"repository"`
}

// RepoStatusRsp is the response message from a successful RepoStatus call.
type RepoStatusRsp struct {
	Status *poll.Status `json:"status"`
}
