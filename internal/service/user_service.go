package service

import (
	"aeibi/api"
	"aeibi/internal/config"
	"aeibi/internal/repository/db"
	"aeibi/internal/repository/oss"
	"aeibi/util"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path"
	"strconv"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	db  *db.Queries
	dbx *sql.DB
	oss *oss.OSS
	cfg *config.Config
}

func NewUserService(dbx *sql.DB, ossClient *oss.OSS, cfg *config.Config) *UserService {
	return &UserService{
		db:  db.New(dbx),
		dbx: dbx,
		oss: ossClient,
		cfg: cfg,
	}
}

func (s *UserService) CreateUser(ctx context.Context, req *api.CreateUserRequest) (*api.CreateUserResponse, error) {
	var resp *api.CreateUserResponse
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		uid := uuid.NewString()

		avatar, err := util.GenerateDefaultAvatar(uid)
		if err != nil {
			return fmt.Errorf("generate default avatar: %w", err)
		}
		avatarKey := fmt.Sprintf("avatars/%s.png", uid)
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}
		row, err := qtx.CreateUser(ctx, db.CreateUserParams{
			Uid:          uid,
			Username:     req.Username,
			Email:        req.Email,
			Nickname:     req.Nickname,
			PasswordHash: string(passwordHash),
			AvatarUrl:    avatarKey,
		})
		if err != nil {
			return fmt.Errorf("create user: %w", err)
		}
		if avatarKey, err = s.oss.PutObject(ctx, avatarKey, avatar, "image/png"); err != nil {
			return fmt.Errorf("upload avatar: %w", err)
		}

		avatarRow, err := qtx.CreateFile(ctx, db.CreateFileParams{
			Url:         avatarKey,
			Name:        path.Base(avatarKey),
			ContentType: "image/png",
			Size:        int64(len(avatar)),
			Checksum:    util.SHA256(avatar),
			Uploader:    row.Uid,
		})
		if err != nil {
			return fmt.Errorf("save file: %w", err)
		}
		resp = &api.CreateUserResponse{
			User: &api.User{
				Uid:       row.Uid,
				Username:  row.Username,
				Email:     row.Email,
				Nickname:  row.Nickname,
				AvatarUrl: avatarRow.Url,
			},
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *UserService) ListUsers(ctx context.Context, req *api.ListUsersRequest) (*api.ListUsersResponse, error) {
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	} else if pageSize > 100 {
		pageSize = 100
	}

	var offset int64
	if req.PageToken != "" {
		o, err := strconv.ParseInt(req.PageToken, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid page token: %w", err)
		}
		if o > 0 {
			offset = o
		}
	}

	total, err := s.db.CountUsers(ctx, req.Filter)
	if err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}

	rows, err := s.db.ListUsers(ctx, db.ListUsersParams{
		Filter: req.Filter,
		Offset: offset,
		Limit:  int64(pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	users := make([]*api.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, &api.User{
			Uid:       row.Uid,
			Username:  row.Username,
			Role:      row.Role,
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		})
	}

	nextPageToken := ""
	if offset+int64(len(rows)) < total {
		nextPageToken = strconv.FormatInt(offset+int64(len(rows)), 10)
	}

	return &api.ListUsersResponse{
		Users:         users,
		NextPageToken: nextPageToken,
		TotalSize:     total,
	}, nil
}

func (s *UserService) GetUser(ctx context.Context, req *api.GetUserRequest) (*api.GetUserResponse, error) {
	row, err := s.db.GetUserByUid(ctx, req.Uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &api.GetUserResponse{
		User: &api.User{
			Uid:       row.Uid,
			Username:  row.Username,
			Role:      row.Role,
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		},
	}, nil
}

func (s *UserService) UpdateUser(ctx context.Context, req *api.UpdateUserRequest) (*api.UpdateUserResponse, error) {
	params := db.UpdateUserByUidParams{Uid: req.Uid}

	if req.Username != nil {
		params.Username = s.nsPtr(req.Username)
	}
	if req.Email != nil {
		params.Email = s.nsPtr(req.Email)
	}
	if req.Nickname != nil {
		params.Nickname = s.nsPtr(req.Nickname)
	}
	if req.AvatarUrl != nil {
		params.AvatarUrl = s.nsPtr(req.AvatarUrl)
	}

	row, err := s.db.UpdateUserByUid(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &api.UpdateUserResponse{
		User: &api.User{
			Uid:       row.Uid,
			Username:  row.Username,
			Role:      row.Role,
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		},
	}, nil
}

func (s *UserService) DeleteUser(ctx context.Context, req *api.DeleteUserRequest) error {
	return db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		affected, err := qtx.ArchiveUserByUid(ctx, req.Uid)
		if err != nil {
			return fmt.Errorf("archive user: %w", err)
		}
		if affected == 0 {
			return fmt.Errorf("user not found or already archived")
		}
		return nil
	})
}

func (s *UserService) GetMe(ctx context.Context, uid string) (*api.GetMeResponse, error) {
	row, err := s.db.GetUserByUid(ctx, uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &api.GetMeResponse{
		User: &api.User{
			Uid:       row.Uid,
			Username:  row.Username,
			Role:      row.Role,
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		},
	}, nil
}

func (s *UserService) UpdateMe(ctx context.Context, uid string, req *api.UpdateMeRequest) (*api.UpdateMeResponse, error) {
	params := db.UpdateUserByUidParams{Uid: uid}

	if req.Username != nil {
		params.Username = s.nsPtr(req.Username)
	}
	if req.Email != nil {
		params.Email = s.nsPtr(req.Email)
	}
	if req.Nickname != nil {
		params.Nickname = s.nsPtr(req.Nickname)
	}
	if req.AvatarUrl != nil {
		params.AvatarUrl = s.nsPtr(req.AvatarUrl)
	}

	row, err := s.db.UpdateUserByUid(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("update user: %w", err)
	}

	return &api.UpdateMeResponse{
		User: &api.User{
			Uid:       row.Uid,
			Username:  row.Username,
			Role:      row.Role,
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		},
	}, nil
}

func (s *UserService) Login(ctx context.Context, req *api.LoginRequest) (*api.LoginResponse, error) {
	var resp *api.LoginResponse
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		row, err := qtx.GetUserAuthByAccount(ctx, req.Account)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("invalid credentials")
			}
			return fmt.Errorf("get user: %w", err)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(req.Password)); err != nil {
			return fmt.Errorf("invalid credentials")
		}

		accessToken, refreshToken, err := s.genToken(row.Uid)
		if err != nil {
			return err
		}

		if _, err := qtx.UpsertRefreshToken(ctx, db.UpsertRefreshTokenParams{
			Uid:       row.Uid,
			Token:     refreshToken,
			ExpiresAt: time.Now().Add(s.cfg.Auth.RefreshTTL).Unix(),
		}); err != nil {
			return fmt.Errorf("save refresh token: %w", err)
		}

		resp = &api.LoginResponse{
			Tokens: &api.TokenPair{
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			},
			User: &api.User{
				Uid:       row.Uid,
				Username:  row.Username,
				Role:      row.Role,
				Email:     row.Email,
				Nickname:  row.Nickname,
				AvatarUrl: row.AvatarUrl,
			},
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserService) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*api.RefreshTokenResponse, error) {
	var resp *api.RefreshTokenResponse
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		row, err := qtx.GetRefreshTokenByToken(ctx, req.RefreshToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("invalid refresh token")
			}
			return fmt.Errorf("get refresh token: %w", err)
		}

		now := time.Now()
		if now.Unix() >= row.ExpiresAt {
			return fmt.Errorf("refresh token expired")
		}

		uid := row.Uid
		accessToken, refreshToken, err := s.genToken(uid)
		if err != nil {
			return err
		}

		if _, err := qtx.UpsertRefreshToken(ctx, db.UpsertRefreshTokenParams{
			Uid:       uid,
			Token:     refreshToken,
			ExpiresAt: now.Add(s.cfg.Auth.RefreshTTL).Unix(),
		}); err != nil {
			return fmt.Errorf("save refresh token: %w", err)
		}

		resp = &api.RefreshTokenResponse{
			Tokens: &api.TokenPair{
				AccessToken:  accessToken,
				RefreshToken: refreshToken,
			},
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *UserService) ChangePassword(ctx context.Context, uid string, req *api.ChangePasswordRequest) error {
	return db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		oldHash, err := qtx.GetUserPasswordHashByUid(ctx, uid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("user not found")
			}
			return fmt.Errorf("get password: %w", err)
		}

		if err := bcrypt.CompareHashAndPassword([]byte(oldHash), []byte(req.OldPassword)); err != nil {
			return fmt.Errorf("old password incorrect")
		}

		newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("hash password: %w", err)
		}

		if _, err := qtx.UpdateUserPasswordHashByUid(ctx, db.UpdateUserPasswordHashByUidParams{
			PasswordHash: string(newHash),
			Uid:          uid,
		}); err != nil {
			return fmt.Errorf("update password: %w", err)
		}

		return nil
	})
}

func (s *UserService) nsPtr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}

func (s *UserService) genToken(uid string) (string, string, error) {
	accessToken, err := util.GenerateJWT(uid, s.cfg.Auth.JWTSecret, s.cfg.Auth.JWTIssuer, s.cfg.Auth.JWTTTL)
	if err != nil {
		return "", "", fmt.Errorf("generate access token: %w", err)
	}
	refreshToken, err := util.RandomString64()
	if err != nil {
		return "", "", fmt.Errorf("generate refresh token: %w", err)
	}
	return accessToken, refreshToken, nil
}
