package service

import (
	"aeibi/api"
	"aeibi/internal/repository/db"
	"aeibi/internal/repository/oss"
	"aeibi/util"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PostService struct {
	db  *db.Queries
	dbx *sql.DB
	oss *oss.OSS
}

func NewPostService(dbx *sql.DB, ossClient *oss.OSS) *PostService {
	return &PostService{
		db:  db.New(dbx),
		dbx: dbx,
		oss: ossClient,
	}
}

func (s *PostService) CreatePost(ctx context.Context, uid string, req *api.CreatePostRequest) (*api.CreatePostResponse, error) {
	var resp *api.CreatePostResponse
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		row, err := qtx.CreatePost(ctx, db.CreatePostParams{
			Author:      util.UUID(uid),
			Text:        req.Text,
			Images:      req.Images,
			Attachments: req.Attachments,
			Visibility:  db.PostVisibility(req.Visibility),
			Pinned:      req.Pinned,
		})
		if err != nil {
			return fmt.Errorf("create post: %w", err)
		}
		err = qtx.UpsertPostTags(ctx, db.UpsertPostTagsParams{
			PostID: row.ID,
			Tags:   util.NormalizeStrings(req.Tags),
		})
		if err != nil {
			return fmt.Errorf("create post: %w", err)
		}
		resp.Uid = row.Uid.String()
		return nil
	}); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *PostService) GetPost(ctx context.Context, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	postRow, err := s.db.GetPostByUid(ctx, util.UUID(req.Uid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	if postRow.Visibility == db.PostVisibilityPRIVATE {
		return nil, fmt.Errorf("post not found")
	}
	fileRow, err := s.db.GetFilesByUrls(ctx, postRow.Attachments)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get attachments: %w", err)
	}
	attachments := make([]*api.Attachment, 0, len(postRow.Attachments))
	for _, file := range fileRow {
		attachments = append(attachments, &api.Attachment{
			Url:         file.Url,
			Name:        file.Name,
			ContentType: file.ContentType,
			Size:        file.Size,
			Checksum:    file.Checksum,
		})
	}
	return &api.GetPostResponse{Post: &api.Post{
		Uid: postRow.Uid.String(),
		Author: &api.PostAuthor{
			Uid:       postRow.AuthorUid.String(),
			Nickname:  postRow.AuthorNickname,
			AvatarUrl: postRow.AuthorAvatarUrl,
		},
		Text:            postRow.Text,
		Images:          postRow.Images,
		Attachments:     attachments,
		Tags:            postRow.TagNames,
		CommentCount:    int64(postRow.CommentCount),
		CollectionCount: int64(postRow.CollectionCount),
		LikeCount:       int64(postRow.LikeCount),
		Visibility:      string(postRow.Visibility),
		LatestRepliedOn: postRow.LatestRepliedOn.Unix(),
		Ip:              postRow.Ip,
		Pinned:          postRow.Pinned,
		CreatedAt:       postRow.CreatedAt.Unix(),
		UpdatedAt:       postRow.UpdatedAt.Unix(),
	}}, nil
}

func (s *PostService) GetMyPost(ctx context.Context, uid string, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	postRow, err := s.db.GetPostByUid(ctx, util.UUID(req.Uid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("get post: %w", err)
	}
	if postRow.Visibility == db.PostVisibilityPRIVATE && uid != postRow.Author.String() {
		return nil, fmt.Errorf("post not found")
	}
	fileRow, err := s.db.GetFilesByUrls(ctx, postRow.Attachments)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("get attachments: %w", err)
	}
	attachments := make([]*api.Attachment, 0, len(postRow.Attachments))
	for _, file := range fileRow {
		attachments = append(attachments, &api.Attachment{
			Url:         file.Url,
			Name:        file.Name,
			ContentType: file.ContentType,
			Size:        file.Size,
			Checksum:    file.Checksum,
		})
	}
	return &api.GetPostResponse{Post: &api.Post{
		Uid: postRow.Uid.String(),
		Author: &api.PostAuthor{
			Uid:       postRow.AuthorUid.String(),
			Nickname:  postRow.AuthorNickname,
			AvatarUrl: postRow.AuthorAvatarUrl,
		},
		Text:            postRow.Text,
		Images:          postRow.Images,
		Attachments:     attachments,
		Tags:            postRow.TagNames,
		CommentCount:    int64(postRow.CommentCount),
		CollectionCount: int64(postRow.CollectionCount),
		LikeCount:       int64(postRow.LikeCount),
		Visibility:      string(postRow.Visibility),
		LatestRepliedOn: postRow.LatestRepliedOn.Unix(),
		Ip:              postRow.Ip,
		Pinned:          postRow.Pinned,
		CreatedAt:       postRow.CreatedAt.Unix(),
		UpdatedAt:       postRow.UpdatedAt.Unix(),
	}}, nil
}

func (s *PostService) ListPosts(ctx context.Context, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	rows, err := s.db.ListPosts(ctx, db.ListPostsParams{
		CursorCreatedAt: sql.NullTime{Time: time.Unix(req.CursorCreatedAt, 0).UTC(), Valid: req.CursorCreatedAt != 0},
		CursorID:        uuid.NullUUID{UUID: util.UUID(req.CursorId), Valid: req.CursorId != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		if row.Visibility == db.PostVisibilityPRIVATE {
			continue
		}

		fileRow, err := s.db.GetFilesByUrls(ctx, row.Attachments)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			continue
		}
		attachments := make([]*api.Attachment, 0, len(row.Attachments))
		for _, file := range fileRow {
			attachments = append(attachments, &api.Attachment{
				Url:         file.Url,
				Name:        file.Name,
				ContentType: file.ContentType,
				Size:        file.Size,
				Checksum:    file.Checksum,
			})
		}

		posts = append(posts, &api.Post{
			Uid: row.Uid.String(),
			Author: &api.PostAuthor{
				Uid:       row.AuthorUid.String(),
				Nickname:  row.AuthorNickname,
				AvatarUrl: row.AuthorAvatarUrl,
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    int64(row.CommentCount),
			CollectionCount: int64(row.CollectionCount),
			LikeCount:       int64(row.LikeCount),
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			CreatedAt:       row.CreatedAt.Unix(),
			UpdatedAt:       row.UpdatedAt.Unix(),
		})
	}

	var nextCursorCreatedAt int64
	var nextCursorID string
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursorCreatedAt = last.CreatedAt.Unix()
		nextCursorID = last.Uid.String()
	}

	return &api.ListPostsResponse{
		Posts:               posts,
		NextCursorCreatedAt: nextCursorCreatedAt,
		NextCursorId:        nextCursorID,
	}, nil
}

func (s *PostService) ListPostsByAuthor(ctx context.Context, req *api.ListPostsByAuthorRequest) (*api.ListPostsResponse, error) {
	rows, err := s.db.ListPostsByAuthor(ctx, db.ListPostsByAuthorParams{
		Author:          util.UUID(req.Uid),
		CursorCreatedAt: sql.NullTime{Time: time.Unix(req.CursorCreatedAt, 0).UTC(), Valid: req.CursorCreatedAt != 0},
		CursorID:        uuid.NullUUID{UUID: util.UUID(req.CursorId), Valid: req.CursorId != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		if row.Visibility == db.PostVisibilityPRIVATE {
			continue
		}

		fileRow, err := s.db.GetFilesByUrls(ctx, row.Attachments)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			continue
		}

		attachments := make([]*api.Attachment, 0, len(row.Attachments))
		for _, file := range fileRow {
			attachments = append(attachments, &api.Attachment{
				Url:         file.Url,
				Name:        file.Name,
				ContentType: file.ContentType,
				Size:        file.Size,
				Checksum:    file.Checksum,
			})
		}

		posts = append(posts, &api.Post{
			Uid: row.Uid.String(),
			Author: &api.PostAuthor{
				Uid:       row.AuthorUid.String(),
				Nickname:  row.AuthorNickname,
				AvatarUrl: row.AuthorAvatarUrl,
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    int64(row.CommentCount),
			CollectionCount: int64(row.CollectionCount),
			LikeCount:       int64(row.LikeCount),
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			CreatedAt:       row.CreatedAt.Unix(),
			UpdatedAt:       row.UpdatedAt.Unix(),
		})
	}

	var nextCursorCreatedAt int64
	var nextCursorID string
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursorCreatedAt = last.CreatedAt.Unix()
		nextCursorID = last.Uid.String()
	}

	return &api.ListPostsResponse{
		Posts:               posts,
		NextCursorCreatedAt: nextCursorCreatedAt,
		NextCursorId:        nextCursorID,
	}, nil
}

func (s *PostService) ListMyPosts(ctx context.Context, uid string, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	rows, err := s.db.ListPostsByAuthor(ctx, db.ListPostsByAuthorParams{
		Author:          util.UUID(uid),
		CursorCreatedAt: sql.NullTime{Time: time.Unix(req.CursorCreatedAt, 0).UTC(), Valid: req.CursorCreatedAt != 0},
		CursorID:        uuid.NullUUID{UUID: util.UUID(req.CursorId), Valid: req.CursorId != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		fileRow, err := s.db.GetFilesByUrls(ctx, row.Attachments)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			continue
		}

		attachments := make([]*api.Attachment, 0, len(row.Attachments))
		for _, file := range fileRow {
			attachments = append(attachments, &api.Attachment{
				Url:         file.Url,
				Name:        file.Name,
				ContentType: file.ContentType,
				Size:        file.Size,
				Checksum:    file.Checksum,
			})
		}

		posts = append(posts, &api.Post{
			Uid: row.Uid.String(),
			Author: &api.PostAuthor{
				Uid:       row.AuthorUid.String(),
				Nickname:  row.AuthorNickname,
				AvatarUrl: row.AuthorAvatarUrl,
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    int64(row.CommentCount),
			CollectionCount: int64(row.CollectionCount),
			LikeCount:       int64(row.LikeCount),
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			CreatedAt:       row.CreatedAt.Unix(),
			UpdatedAt:       row.UpdatedAt.Unix(),
		})
	}

	var nextCursorCreatedAt int64
	var nextCursorID string
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursorCreatedAt = last.CreatedAt.Unix()
		nextCursorID = last.Uid.String()
	}

	return &api.ListPostsResponse{
		Posts:               posts,
		NextCursorCreatedAt: nextCursorCreatedAt,
		NextCursorId:        nextCursorID,
	}, nil
}

func (s *PostService) ListMyCollections(ctx context.Context, uid string, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	rows, err := s.db.ListPostsByCollector(ctx, db.ListPostsByCollectorParams{
		Collector:       util.UUID(uid),
		CursorCreatedAt: sql.NullTime{Time: time.Unix(req.CursorCreatedAt, 0).UTC(), Valid: req.CursorCreatedAt != 0},
		CursorID:        uuid.NullUUID{UUID: util.UUID(req.CursorId), Valid: req.CursorId != ""},
	})
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		if row.Visibility == db.PostVisibilityPRIVATE && uid != row.Author.String() {
			continue
		}

		fileRow, err := s.db.GetFilesByUrls(ctx, row.Attachments)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			continue
		}

		attachments := make([]*api.Attachment, 0, len(row.Attachments))
		for _, file := range fileRow {
			attachments = append(attachments, &api.Attachment{
				Url:         file.Url,
				Name:        file.Name,
				ContentType: file.ContentType,
				Size:        file.Size,
				Checksum:    file.Checksum,
			})
		}

		posts = append(posts, &api.Post{
			Uid: row.Uid.String(),
			Author: &api.PostAuthor{
				Uid:       row.AuthorUid.String(),
				Nickname:  row.AuthorNickname,
				AvatarUrl: row.AuthorAvatarUrl,
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    int64(row.CommentCount),
			CollectionCount: int64(row.CollectionCount),
			LikeCount:       int64(row.LikeCount),
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			CreatedAt:       row.CreatedAt.Unix(),
			UpdatedAt:       row.UpdatedAt.Unix(),
		})
	}

	var nextCursorCreatedAt int64
	var nextCursorID string
	if len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursorCreatedAt = last.CreatedAt.Unix()
		nextCursorID = last.Uid.String()
	}

	return &api.ListPostsResponse{
		Posts:               posts,
		NextCursorCreatedAt: nextCursorCreatedAt,
		NextCursorId:        nextCursorID,
	}, nil
}

func (s *PostService) UpdatePost(ctx context.Context, uid string, req *api.UpdatePostRequest) error {
	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		params := db.UpdatePostByUidAndAuthorParams{
			Uid:         util.UUID(req.Uid),
			Author:      util.UUID(uid),
			Images:      req.Images,
			Attachments: req.Attachments,
		}
		if req.Text != nil {
			params.Text = sql.NullString{String: *req.Text, Valid: true}
		}
		if req.Visibility != nil {
			params.Visibility = db.NullPostVisibility{PostVisibility: db.PostVisibility(*req.Visibility), Valid: true}
		}
		if req.Pinned != nil {
			params.Pinned = sql.NullBool{Bool: *req.Pinned, Valid: true}
		}

		id, err := qtx.UpdatePostByUidAndAuthor(ctx, params)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("post not found")
			}
			return fmt.Errorf("update post: %w", err)
		}
		err = qtx.UpsertPostTags(ctx, db.UpsertPostTagsParams{
			PostID: id,
			Tags:   util.NormalizeStrings(req.Tags),
		})
		if err != nil {
			return fmt.Errorf("update post: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}
	return nil
}

func (s *PostService) DeletePost(ctx context.Context, uid string, req *api.DeletePostRequest) error {
	return db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		affected, err := qtx.ArchivePostByUidAndAuthor(ctx, db.ArchivePostByUidAndAuthorParams{
			Uid:    util.UUID(req.Uid),
			Author: util.UUID(uid),
		})
		if err != nil {
			return fmt.Errorf("archive post: %w", err)
		}
		if affected == 0 {
			return fmt.Errorf("post not found or no permission")
		}
		return nil
	})
}

func (s *PostService) LikePost(ctx context.Context, uid string, req *api.LikePostRequest) error {
	return nil
}

func (s *PostService) CollectPost(ctx context.Context, uid string, req *api.CollectPostRequest) error {
	return nil
}
