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
			Visibility:  db.NullPostVisibility{PostVisibility: db.PostVisibility(req.Visibility), Valid: req.Visibility != ""},
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
		resp = &api.CreatePostResponse{
			Uid: row.Uid.String(),
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return resp, nil
}

func (s *PostService) GetPost(ctx context.Context, viewerUid string, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	postRow, err := s.db.GetPostByUid(ctx, db.GetPostByUidParams{
		Uid:    util.UUID(req.Uid),
		Viewer: uuid.NullUUID{UUID: util.UUID(viewerUid), Valid: viewerUid != ""},
	})
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
			Uid:         postRow.AuthorUid.String(),
			Nickname:    postRow.AuthorNickname,
			AvatarUrl:   postRow.AuthorAvatarUrl,
			IsFollowing: false, // TODO: compute with viewer context
		},
		Text:            postRow.Text,
		Images:          postRow.Images,
		Attachments:     attachments,
		Tags:            postRow.TagNames,
		CommentCount:    postRow.CommentCount,
		CollectionCount: postRow.CollectionCount,
		LikeCount:       postRow.LikeCount,
		Visibility:      string(postRow.Visibility),
		LatestRepliedOn: postRow.LatestRepliedOn.Unix(),
		Ip:              postRow.Ip,
		Pinned:          postRow.Pinned,
		Liked:           postRow.Liked,
		Collected:       postRow.Collected,
		CreatedAt:       postRow.CreatedAt.Unix(),
		UpdatedAt:       postRow.UpdatedAt.Unix(),
	}}, nil
}

func (s *PostService) GetMyPost(ctx context.Context, uid string, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	postRow, err := s.db.GetPostByUid(ctx, db.GetPostByUidParams{
		Uid:    util.UUID(req.Uid),
		Viewer: uuid.NullUUID{UUID: util.UUID(uid), Valid: true},
	})
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
			Uid:         postRow.AuthorUid.String(),
			Nickname:    postRow.AuthorNickname,
			AvatarUrl:   postRow.AuthorAvatarUrl,
			IsFollowing: false, // TODO: compute with viewer context
		},
		Text:            postRow.Text,
		Images:          postRow.Images,
		Attachments:     attachments,
		Tags:            postRow.TagNames,
		CommentCount:    postRow.CommentCount,
		CollectionCount: postRow.CollectionCount,
		LikeCount:       postRow.LikeCount,
		Visibility:      string(postRow.Visibility),
		LatestRepliedOn: postRow.LatestRepliedOn.Unix(),
		Ip:              postRow.Ip,
		Pinned:          postRow.Pinned,
		Liked:           postRow.Liked,
		Collected:       postRow.Collected,
		CreatedAt:       postRow.CreatedAt.Unix(),
		UpdatedAt:       postRow.UpdatedAt.Unix(),
	}}, nil
}

func (s *PostService) ListPosts(ctx context.Context, viewerUid string, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
	rows, err := s.db.ListPosts(ctx, db.ListPostsParams{
		Viewer:          uuid.NullUUID{UUID: util.UUID(viewerUid), Valid: viewerUid != ""},
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
				Uid:         row.AuthorUid.String(),
				Nickname:    row.AuthorNickname,
				AvatarUrl:   row.AuthorAvatarUrl,
				IsFollowing: false, // TODO: compute with viewer context
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			Liked:           row.Liked,
			Collected:       row.Collected,
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

func (s *PostService) ListPostsByAuthor(ctx context.Context, viewerUid string, req *api.ListPostsByAuthorRequest) (*api.ListPostsResponse, error) {
	rows, err := s.db.ListPostsByAuthor(ctx, db.ListPostsByAuthorParams{
		Author:          util.UUID(req.Uid),
		Viewer:          uuid.NullUUID{UUID: util.UUID(viewerUid), Valid: viewerUid != ""},
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
				Uid:         row.AuthorUid.String(),
				Nickname:    row.AuthorNickname,
				AvatarUrl:   row.AuthorAvatarUrl,
				IsFollowing: false, // TODO: compute with viewer context
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			Liked:           row.Liked,
			Collected:       row.Collected,
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
		Viewer:          uuid.NullUUID{UUID: util.UUID(uid), Valid: true},
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
				Uid:         row.AuthorUid.String(),
				Nickname:    row.AuthorNickname,
				AvatarUrl:   row.AuthorAvatarUrl,
				IsFollowing: false, // TODO: compute with viewer context
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			Liked:           row.Liked,
			Collected:       row.Collected,
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
				Uid:         row.AuthorUid.String(),
				Nickname:    row.AuthorNickname,
				AvatarUrl:   row.AuthorAvatarUrl,
				IsFollowing: false, // TODO: compute with viewer context
			},
			Text:            row.Text,
			Images:          row.Images,
			Attachments:     attachments,
			Tags:            row.TagNames,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      string(row.Visibility),
			LatestRepliedOn: row.LatestRepliedOn.Unix(),
			Ip:              row.Ip,
			Pinned:          row.Pinned,
			Liked:           row.Liked,
			Collected:       row.Collected,
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
			Uid:    util.UUID(req.Uid),
			Author: util.UUID(uid),
		}
		paths := make(map[string]struct{}, len(req.UpdateMask.GetPaths()))
		for _, path := range req.UpdateMask.GetPaths() {
			paths[path] = struct{}{}
		}
		if _, ok := paths["text"]; ok {
			params.Text = sql.NullString{String: req.Post.Text, Valid: true}
		}
		if _, ok := paths["images"]; ok {
			params.Images = req.Post.Images
		}
		if _, ok := paths["attachments"]; ok {
			params.Attachments = req.Post.Attachments
		}
		if _, ok := paths["visibility"]; ok {
			params.Visibility = db.NullPostVisibility{PostVisibility: db.PostVisibility(req.Post.Visibility), Valid: true}
		}
		if _, ok := paths["pinned"]; ok {
			params.Pinned = sql.NullBool{Bool: req.Post.Pinned, Valid: true}
		}

		id, err := qtx.UpdatePostByUidAndAuthor(ctx, params)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("post not found")
			}
			return fmt.Errorf("update post: %w", err)
		}
		if _, ok := paths["tags"]; ok {
			err = qtx.UpsertPostTags(ctx, db.UpsertPostTagsParams{
				PostID: id,
				Tags:   util.NormalizeStrings(req.Post.Tags),
			})
			if err != nil {
				return fmt.Errorf("update post: %w", err)
			}
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

func (s *PostService) LikePost(ctx context.Context, uid string, req *api.LikePostRequest) (*api.LikePostResponse, error) {
	postUid := util.UUID(req.Uid)
	userUid := util.UUID(uid)

	var (
		count int32
		err   error
	)

	switch req.Action {
	case api.ToggleAction_TOGGLE_ACTION_ADD:
		count, err = s.db.AddPostLike(ctx, db.AddPostLikeParams{
			PostUid: postUid,
			UserUid: userUid,
		})
	default:
		count, err = s.db.RemovePostLike(ctx, db.RemovePostLikeParams{
			PostUid: postUid,
			UserUid: userUid,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("post like: %w", err)
	}
	return &api.LikePostResponse{
		Count: count,
	}, nil
}

func (s *PostService) CollectPost(ctx context.Context, uid string, req *api.CollectPostRequest) (*api.CollectPostResponse, error) {
	postUid := util.UUID(req.Uid)
	userUid := util.UUID(uid)

	var (
		count int32
		err   error
	)

	switch req.Action {
	case api.ToggleAction_TOGGLE_ACTION_ADD:
		count, err = s.db.AddPostCollection(ctx, db.AddPostCollectionParams{
			PostUid: postUid,
			UserUid: userUid,
		})
	default:
		count, err = s.db.RemovePostCollection(ctx, db.RemovePostCollectionParams{
			PostUid: postUid,
			UserUid: userUid,
		})
	}
	if err != nil {
		return nil, fmt.Errorf("post collection: %w", err)
	}
	return &api.CollectPostResponse{
		Count: count,
	}, nil
}
