package controller

import (
	"aeibi/api"
	"aeibi/internal/auth"
	"aeibi/internal/service"
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type PostHandler struct {
	api.UnimplementedPostServiceServer
	svc *service.PostService
}

func NewPostHandler(svc *service.PostService) *PostHandler {
	return &PostHandler{svc: svc}
}

func (h *PostHandler) CreatePost(ctx context.Context, req *api.CreatePostRequest) (*api.CreatePostResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Text == "" {
		return nil, status.Error(codes.InvalidArgument, "content is required")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return h.svc.CreatePost(ctx, uid, req)
}

func (h *PostHandler) ListPosts(ctx context.Context, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	return h.svc.ListPosts(ctx, req)
}

func (h *PostHandler) ListMyPosts(ctx context.Context, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return h.svc.ListMyPosts(ctx, uid, req)
}

func (h *PostHandler) ListMyCollections(ctx context.Context, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return h.svc.ListMyCollections(ctx, uid, req)
}

func (h *PostHandler) GetMyPost(ctx context.Context, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return h.svc.GetMyPost(ctx, uid, req)
}

func (h *PostHandler) GetPost(ctx context.Context, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	return h.svc.GetPost(ctx, req)
}

func (h *PostHandler) UpdatePost(ctx context.Context, req *api.UpdatePostRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if req.Text == nil && req.Images == nil && req.Attachments == nil && req.Tags == nil && req.Visibility == nil && req.Pinned == nil {
		return nil, status.Error(codes.InvalidArgument, "no fields to update")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if err := h.svc.UpdatePost(ctx, uid, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *PostHandler) DeletePost(ctx context.Context, req *api.DeletePostRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if err := h.svc.DeletePost(ctx, uid, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *PostHandler) LikePost(ctx context.Context, req *api.LikePostRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if err := h.svc.LikePost(ctx, uid, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *PostHandler) CollectPost(ctx context.Context, req *api.CollectPostRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok || uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if err := h.svc.CollectPost(ctx, uid, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
