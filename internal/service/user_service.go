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

func (s *UserService) CreateUser(ctx context.Context, req *api.CreateUserRequest) error {
	uid := uuid.New()
	avatar, err := util.GenerateDefaultAvatar(uid.String())
	if err != nil {
		return fmt.Errorf("generate default avatar: %w", err)
	}
	avatarKey := fmt.Sprintf("avatars/%s.png", uid)
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		err = qtx.CreateUser(ctx, db.CreateUserParams{
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
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *UserService) GetUser(ctx context.Context, req *api.GetUserRequest) (*api.GetUserResponse, error) {
	row, err := s.db.GetUserByUid(ctx, util.UUID(req.Uid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return &api.GetUserResponse{
		User: &api.User{
			Uid:       row.Uid.String(),
			Username:  row.Username,
			Role:      string(row.Role),
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		},
	}, nil
}

func (s *UserService) GetMe(ctx context.Context, uid string) (*api.GetMeResponse, error) {
	row, err := s.db.GetUserByUid(ctx, util.UUID(uid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	return &api.GetMeResponse{
		User: &api.User{
			Uid:       row.Uid.String(),
			Username:  row.Username,
			Role:      string(row.Role),
			Email:     row.Email,
			Nickname:  row.Nickname,
			AvatarUrl: row.AvatarUrl,
		},
	}, nil
}

func (s *UserService) UpdateMe(ctx context.Context, uid string, req *api.UpdateMeRequest) error {
	params := db.UpdateUserParams{Uid: util.UUID(uid)}
	params.Username = s.nsPtr(req.Username)
	params.Email = s.nsPtr(req.Email)
	params.Nickname = s.nsPtr(req.Nickname)
	params.AvatarUrl = s.nsPtr(req.AvatarUrl)
	err := s.db.UpdateUser(ctx, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user not found")
		}
		return fmt.Errorf("update user: %w", err)
	}
	return nil
}

func (s *UserService) Login(ctx context.Context, req *api.LoginRequest) (*api.LoginResponse, error) {
	var resp *api.LoginResponse
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		row, err := qtx.GetUserByUsername(ctx, req.Account)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("invalid credentials")
			}
			return fmt.Errorf("get user: %w", err)
		}
		if err := bcrypt.CompareHashAndPassword([]byte(row.PasswordHash), []byte(req.Password)); err != nil {
			return fmt.Errorf("invalid credentials")
		}
		accessToken, refreshToken, err := s.genToken(row.Uid.String())
		if err != nil {
			return err
		}

		if err := qtx.UpsertRefreshToken(ctx, db.UpsertRefreshTokenParams{
			Uid:       row.Uid,
			Token:     refreshToken,
			ExpiresAt: time.Now().Add(s.cfg.Auth.RefreshTTL),
		}); err != nil {
			return fmt.Errorf("save refresh token: %w", err)
		}

		resp = &api.LoginResponse{
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

func (s *UserService) RefreshToken(ctx context.Context, req *api.RefreshTokenRequest) (*api.RefreshTokenResponse, error) {
	var resp *api.RefreshTokenResponse
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		row, err := qtx.GetRefreshToken(ctx, req.RefreshToken)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("invalid refresh token")
			}
			return fmt.Errorf("get refresh token: %w", err)
		}

		now := time.Now()
		uid := row.Uid
		accessToken, refreshToken, err := s.genToken(uid.String())
		if err != nil {
			return err
		}

		if err := qtx.UpsertRefreshToken(ctx, db.UpsertRefreshTokenParams{
			Uid:       uid,
			Token:     refreshToken,
			ExpiresAt: now.Add(s.cfg.Auth.RefreshTTL),
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
