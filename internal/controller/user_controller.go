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

type UserHandler struct {
	api.UnimplementedUserServiceServer
	svc *service.UserService
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{svc: svc}
}

func (h *UserHandler) CreateUser(ctx context.Context, req *api.CreateUserRequest) (*api.CreateUserResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Username == "" {
		return nil, status.Error(codes.InvalidArgument, "username is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	return h.svc.CreateUser(ctx, req)
}

func (h *UserHandler) ListUsers(ctx context.Context, req *api.ListUsersRequest) (*api.ListUsersResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	return h.svc.ListUsers(ctx, req)
}

func (h *UserHandler) GetUser(ctx context.Context, req *api.GetUserRequest) (*api.GetUserResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	return h.svc.GetUser(ctx, req)
}

func (h *UserHandler) UpdateUser(ctx context.Context, req *api.UpdateUserRequest) (*api.UpdateUserResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if req.Username == nil || req.Email == nil || req.Nickname == nil || req.AvatarUrl == nil {
		return nil, status.Error(codes.InvalidArgument, "no fields to update")
	}
	return h.svc.UpdateUser(ctx, req)
}

func (h *UserHandler) DeleteUser(ctx context.Context, req *api.DeleteUserRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Uid == "" {
		return nil, status.Error(codes.InvalidArgument, "uid is required")
	}
	if err := h.svc.DeleteUser(ctx, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (h *UserHandler) GetMe(ctx context.Context, _ *emptypb.Empty) (*api.GetMeResponse, error) {
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return h.svc.GetMe(ctx, uid)
}

func (h *UserHandler) UpdateMe(ctx context.Context, req *api.UpdateMeRequest) (*api.UpdateMeResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Username == nil || req.Email == nil || req.Nickname == nil || req.AvatarUrl == nil {
		return nil, status.Error(codes.InvalidArgument, "no fields to update")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	return h.svc.UpdateMe(ctx, uid, req)
}

func (h *UserHandler) Login(ctx context.Context, req *api.LoginRequest) (*api.LoginResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.Account == "" {
		return nil, status.Error(codes.InvalidArgument, "account is required")
	}
	if req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "password is required")
	}
	return h.svc.Login(ctx, req)
}

func (h *UserHandler) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*api.RefreshTokenResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.RefreshToken == "" {
		return nil, status.Error(codes.InvalidArgument, "refresh_token is required")
	}
	return h.svc.RefreshToken(ctx, req)
}

func (h *UserHandler) ChangePassword(ctx context.Context, req *api.ChangePasswordRequest) (*emptypb.Empty, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is nil")
	}
	if req.OldPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "old_password is required")
	}
	if req.NewPassword == "" {
		return nil, status.Error(codes.InvalidArgument, "new_password is required")
	}
	uid, ok := auth.SubjectFromContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if uid == "" {
		return nil, status.Error(codes.Unauthenticated, "unauthenticated")
	}
	if err := h.svc.ChangePassword(ctx, uid, req); err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}
