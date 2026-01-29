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
	"strconv"

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
		postUID := uuid.NewString()

		images, err := util.EncodeStringSlice(req.Images)
		if err != nil {
			return fmt.Errorf("marshal images: %w", err)
		}
		attachments, err := util.EncodeStringSlice(req.Attachments)
		if err != nil {
			return fmt.Errorf("marshal attachments: %w", err)
		}

		postRow, err := qtx.CreatePost(ctx, db.CreatePostParams{
			Uid:         postUID,
			Author:      uid,
			Text:        req.Text,
			Images:      string(images),
			Attachments: string(attachments),
			Visibility:  req.Visibility,
			Pinned:      util.BoolToInt64(req.Pinned),
			Ip:          "",
		})
		if err != nil {
			return fmt.Errorf("create post: %w", err)
		}

		req.Tags = util.NormalizeStrings(req.Tags)

		for _, tag := range req.Tags {
			tagRow, err := qtx.UpsertTag(ctx, tag)
			if err != nil {
				return fmt.Errorf("upsert tag %q: %w", tag, err)
			}
			if err := qtx.AddPostTag(ctx, db.AddPostTagParams{
				PostID: postRow.ID,
				TagID:  tagRow.ID,
			}); err != nil {
				return fmt.Errorf("attach tag %q: %w", tag, err)
			}
		}

		tags, err := qtx.ListPostTagsByUid(ctx, postRow.Uid)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		postRowWithAuthor, err := qtx.GetPostByUid(ctx, postRow.Uid)
		if err != nil {
			return fmt.Errorf("get post: %w", err)
		}

		post := &api.Post{
			Uid:             postRowWithAuthor.Uid,
			Author:          s.postAuthor(postRowWithAuthor.Author, postRowWithAuthor.AuthorNickname, postRowWithAuthor.AuthorAvatarUrl),
			Text:            postRowWithAuthor.Text,
			Tags:            tags,
			CommentCount:    postRowWithAuthor.CommentCount,
			CollectionCount: postRowWithAuthor.CollectionCount,
			LikeCount:       postRowWithAuthor.LikeCount,
			Visibility:      postRowWithAuthor.Visibility,
			LatestRepliedOn: postRowWithAuthor.LatestRepliedOn,
			Ip:              postRowWithAuthor.Ip,
			Pinned:          postRowWithAuthor.Pinned == 1,
			CreatedAt:       postRowWithAuthor.CreatedAt,
			UpdatedAt:       postRowWithAuthor.UpdatedAt,
		}
		post.Images, err = util.DecodeStringSlice(postRowWithAuthor.Images)
		if err != nil {
			return fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(postRowWithAuthor.Attachments)
		if err != nil {
			return fmt.Errorf("decode attachments: %w", err)
		}
		resp = &api.CreatePostResponse{Post: post}
		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PostService) ListPosts(ctx context.Context, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
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

	total, err := s.db.CountPublicPosts(ctx, db.CountPublicPostsParams{
		Author:     req.Author,
		Visibility: req.Visibility,
		Search:     req.Search,
		Tag:        req.Tag,
	})
	if err != nil {
		return nil, fmt.Errorf("count posts: %w", err)
	}

	rows, err := s.db.ListPublicPosts(ctx, db.ListPublicPostsParams{
		Author:     req.Author,
		Visibility: req.Visibility,
		Search:     req.Search,
		Tag:        req.Tag,
		Offset:     offset,
		Limit:      int64(pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		tags, err := s.db.ListPostTagsByUid(ctx, row.Uid)
		if err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}

		post := &api.Post{
			Uid:             row.Uid,
			Author:          s.postAuthor(row.Author, row.AuthorNickname, row.AuthorAvatarUrl),
			Text:            row.Text,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      row.Visibility,
			LatestRepliedOn: row.LatestRepliedOn,
			Ip:              row.Ip,
			Pinned:          row.Pinned == 1,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
			Tags:            tags,
		}
		post.Images, err = util.DecodeStringSlice(row.Images)
		if err != nil {
			return nil, fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(row.Attachments)
		if err != nil {
			return nil, fmt.Errorf("decode attachments: %w", err)
		}
		posts = append(posts, post)
	}

	nextPageToken := ""
	if offset+int64(len(rows)) < total {
		nextPageToken = strconv.FormatInt(offset+int64(len(rows)), 10)
	}

	return &api.ListPostsResponse{
		Posts:         posts,
		NextPageToken: nextPageToken,
		TotalSize:     total,
	}, nil
}

func (s *PostService) ListMyPosts(ctx context.Context, uid string, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
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

	total, err := s.db.CountMyPosts(ctx, db.CountMyPostsParams{
		Author:     uid,
		Visibility: req.Visibility,
		Search:     req.Search,
		Tag:        req.Tag,
	})
	if err != nil {
		return nil, fmt.Errorf("count posts: %w", err)
	}

	rows, err := s.db.ListMyPosts(ctx, db.ListMyPostsParams{
		Author:     uid,
		Visibility: req.Visibility,
		Search:     req.Search,
		Tag:        req.Tag,
		Offset:     offset,
		Limit:      int64(pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}

	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		tags, err := s.db.ListPostTagsByUid(ctx, row.Uid)
		if err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}

		post := &api.Post{
			Uid:             row.Uid,
			Author:          s.postAuthor(row.Author, row.AuthorNickname, row.AuthorAvatarUrl),
			Text:            row.Text,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      row.Visibility,
			LatestRepliedOn: row.LatestRepliedOn,
			Ip:              row.Ip,
			Pinned:          row.Pinned == 1,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
			Tags:            tags,
		}
		post.Images, err = util.DecodeStringSlice(row.Images)
		if err != nil {
			return nil, fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(row.Attachments)
		if err != nil {
			return nil, fmt.Errorf("decode attachments: %w", err)
		}
		posts = append(posts, post)
	}

	nextPageToken := ""
	if offset+int64(len(rows)) < total {
		nextPageToken = strconv.FormatInt(offset+int64(len(rows)), 10)
	}

	return &api.ListPostsResponse{
		Posts:         posts,
		NextPageToken: nextPageToken,
		TotalSize:     total,
	}, nil
}

func (s *PostService) ListMyCollections(ctx context.Context, uid string, req *api.ListPostsRequest) (*api.ListPostsResponse, error) {
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

	total, err := s.db.CountMyCollections(ctx, db.CountMyCollectionsParams{
		UserUid:    uid,
		Author:     req.Author,
		Visibility: req.Visibility,
		Search:     req.Search,
		Tag:        req.Tag,
	})
	if err != nil {
		return nil, fmt.Errorf("count collections: %w", err)
	}

	rows, err := s.db.ListMyCollections(ctx, db.ListMyCollectionsParams{
		UserUid:    uid,
		Author:     req.Author,
		Visibility: req.Visibility,
		Search:     req.Search,
		Tag:        req.Tag,
		Offset:     offset,
		Limit:      int64(pageSize),
	})
	if err != nil {
		return nil, fmt.Errorf("list collections: %w", err)
	}

	posts := make([]*api.Post, 0, len(rows))
	for _, row := range rows {
		tags, err := s.db.ListPostTagsByUid(ctx, row.Uid)
		if err != nil {
			return nil, fmt.Errorf("list tags: %w", err)
		}

		post := &api.Post{
			Uid:             row.Uid,
			Author:          s.postAuthor(row.Author, row.AuthorNickname, row.AuthorAvatarUrl),
			Text:            row.Text,
			CommentCount:    row.CommentCount,
			CollectionCount: row.CollectionCount,
			LikeCount:       row.LikeCount,
			Visibility:      row.Visibility,
			LatestRepliedOn: row.LatestRepliedOn,
			Ip:              row.Ip,
			Pinned:          row.Pinned == 1,
			CreatedAt:       row.CreatedAt,
			UpdatedAt:       row.UpdatedAt,
			Tags:            tags,
		}
		post.Images, err = util.DecodeStringSlice(row.Images)
		if err != nil {
			return nil, fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(row.Attachments)
		if err != nil {
			return nil, fmt.Errorf("decode attachments: %w", err)
		}
		posts = append(posts, post)
	}

	nextPageToken := ""
	if offset+int64(len(rows)) < total {
		nextPageToken = strconv.FormatInt(offset+int64(len(rows)), 10)
	}

	return &api.ListPostsResponse{
		Posts:         posts,
		NextPageToken: nextPageToken,
		TotalSize:     total,
	}, nil
}

func (s *PostService) GetPost(ctx context.Context, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	row, err := s.db.GetPublicPostByUid(ctx, req.Uid)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("get post: %w", err)
	}

	tags, err := s.db.ListPostTagsByUid(ctx, req.Uid)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	post := &api.Post{
		Uid:             row.Uid,
		Author:          s.postAuthor(row.Author, row.AuthorNickname, row.AuthorAvatarUrl),
		Text:            row.Text,
		CommentCount:    row.CommentCount,
		CollectionCount: row.CollectionCount,
		LikeCount:       row.LikeCount,
		Visibility:      row.Visibility,
		LatestRepliedOn: row.LatestRepliedOn,
		Ip:              row.Ip,
		Pinned:          row.Pinned == 1,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Tags:            tags,
	}

	post.Images, err = util.DecodeStringSlice(row.Images)
	if err != nil {
		return nil, fmt.Errorf("decode images: %w", err)
	}
	post.Attachments, err = util.DecodeStringSlice(row.Attachments)
	if err != nil {
		return nil, fmt.Errorf("decode attachments: %w", err)
	}

	return &api.GetPostResponse{Post: post}, nil
}

func (s *PostService) GetMyPost(ctx context.Context, uid string, req *api.GetPostRequest) (*api.GetPostResponse, error) {
	row, err := s.db.GetPostByUidAndAuthor(ctx, db.GetPostByUidAndAuthorParams{
		Uid:    req.Uid,
		Author: uid,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("post not found")
		}
		return nil, fmt.Errorf("get post: %w", err)
	}

	tags, err := s.db.ListPostTagsByUid(ctx, req.Uid)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}

	post := &api.Post{
		Uid:             row.Uid,
		Author:          s.postAuthor(row.Author, row.AuthorNickname, row.AuthorAvatarUrl),
		Text:            row.Text,
		CommentCount:    row.CommentCount,
		CollectionCount: row.CollectionCount,
		LikeCount:       row.LikeCount,
		Visibility:      row.Visibility,
		LatestRepliedOn: row.LatestRepliedOn,
		Ip:              row.Ip,
		Pinned:          row.Pinned == 1,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
		Tags:            tags,
	}

	post.Images, err = util.DecodeStringSlice(row.Images)
	if err != nil {
		return nil, fmt.Errorf("decode images: %w", err)
	}
	post.Attachments, err = util.DecodeStringSlice(row.Attachments)
	if err != nil {
		return nil, fmt.Errorf("decode attachments: %w", err)
	}

	return &api.GetPostResponse{Post: post}, nil
}

func (s *PostService) UpdatePost(ctx context.Context, uid string, req *api.UpdatePostRequest) (*api.UpdatePostResponse, error) {
	var resp *api.UpdatePostResponse

	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		params := db.UpdatePostByUidAndAuthorParams{
			Uid:    req.Uid,
			Author: uid,
		}

		if req.Text != nil {
			params.Text = s.nsPtr(req.Text)
		}
		if req.Images != nil {
			images, err := util.EncodeStringSlice(req.Images)
			if err != nil {
				return fmt.Errorf("marshal images: %w", err)
			}
			params.Images = sql.NullString{String: images, Valid: true}
		}
		if req.Attachments != nil {
			attachments, err := util.EncodeStringSlice(req.Attachments)
			if err != nil {
				return fmt.Errorf("marshal attachments: %w", err)
			}
			params.Attachments = sql.NullString{String: attachments, Valid: true}
		}
		if req.Visibility != nil {
			params.Visibility = s.nsPtr(req.Visibility)
		}
		if req.Pinned != nil {
			params.Pinned = sql.NullInt64{Int64: util.BoolToInt64(*req.Pinned), Valid: true}
		}

		updated, err := qtx.UpdatePostByUidAndAuthor(ctx, params)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("post not found")
			}
			return fmt.Errorf("update post: %w", err)
		}

		// Refresh tags if provided
		if req.Tags != nil {
			if err := qtx.DeletePostTags(ctx, updated.ID); err != nil {
				return fmt.Errorf("clear tags: %w", err)
			}
			normTags := util.NormalizeStrings(req.Tags)
			for _, tag := range normTags {
				tagRow, err := qtx.UpsertTag(ctx, tag)
				if err != nil {
					return fmt.Errorf("upsert tag %q: %w", tag, err)
				}
				if err := qtx.AddPostTag(ctx, db.AddPostTagParams{
					PostID: updated.ID,
					TagID:  tagRow.ID,
				}); err != nil {
					return fmt.Errorf("attach tag %q: %w", tag, err)
				}
			}
		}

		tags, err := qtx.ListPostTagsByUid(ctx, req.Uid)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		postRow, err := qtx.GetPostByUid(ctx, req.Uid)
		if err != nil {
			return fmt.Errorf("get post: %w", err)
		}

		post := &api.Post{
			Uid:             postRow.Uid,
			Author:          s.postAuthor(postRow.Author, postRow.AuthorNickname, postRow.AuthorAvatarUrl),
			Text:            postRow.Text,
			CommentCount:    postRow.CommentCount,
			CollectionCount: postRow.CollectionCount,
			LikeCount:       postRow.LikeCount,
			Visibility:      postRow.Visibility,
			LatestRepliedOn: postRow.LatestRepliedOn,
			Ip:              postRow.Ip,
			Pinned:          postRow.Pinned == 1,
			CreatedAt:       postRow.CreatedAt,
			UpdatedAt:       postRow.UpdatedAt,
			Tags:            tags,
		}

		post.Images, err = util.DecodeStringSlice(postRow.Images)
		if err != nil {
			return fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(postRow.Attachments)
		if err != nil {
			return fmt.Errorf("decode attachments: %w", err)
		}

		resp = &api.UpdatePostResponse{Post: post}
		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PostService) DeletePost(ctx context.Context, uid string, req *api.DeletePostRequest) error {
	return db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		affected, err := qtx.ArchivePostByUidAndAuthor(ctx, db.ArchivePostByUidAndAuthorParams{
			Uid:    req.Uid,
			Author: uid,
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

func (s *PostService) LikePost(ctx context.Context, uid string, req *api.LikePostRequest) (*api.PostCounterResponse, error) {
	var resp *api.PostCounterResponse

	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		postRow, err := qtx.GetPostByUid(ctx, req.Uid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("post not found")
			}
			return fmt.Errorf("get post: %w", err)
		}

		var delta int64
		if req.Action == api.ToggleAction_TOGGLE_ACTION_REMOVE {
			affected, err := qtx.DeletePostLike(ctx, db.DeletePostLikeParams{
				PostUid: req.Uid,
				UserUid: uid,
			})
			if err != nil {
				return fmt.Errorf("delete like: %w", err)
			}
			delta = -affected
		} else {
			affected, err := qtx.InsertPostLike(ctx, db.InsertPostLikeParams{
				PostUid: req.Uid,
				UserUid: uid,
			})
			if err != nil {
				return fmt.Errorf("insert like: %w", err)
			}
			delta = affected
		}

		if delta != 0 {
			if _, err := qtx.UpdatePostLikeCount(ctx, db.UpdatePostLikeCountParams{
				Delta: delta,
				Uid:   req.Uid,
			}); err != nil {
				return fmt.Errorf("update like count: %w", err)
			}
			postRow.LikeCount += delta
			if postRow.LikeCount < 0 {
				postRow.LikeCount = 0
			}
		}

		tags, err := qtx.ListPostTagsByUid(ctx, req.Uid)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		post := &api.Post{
			Uid:             postRow.Uid,
			Author:          s.postAuthor(postRow.Author, postRow.AuthorNickname, postRow.AuthorAvatarUrl),
			Text:            postRow.Text,
			CommentCount:    postRow.CommentCount,
			CollectionCount: postRow.CollectionCount,
			LikeCount:       postRow.LikeCount,
			Visibility:      postRow.Visibility,
			LatestRepliedOn: postRow.LatestRepliedOn,
			Ip:              postRow.Ip,
			Pinned:          postRow.Pinned == 1,
			CreatedAt:       postRow.CreatedAt,
			UpdatedAt:       postRow.UpdatedAt,
			Tags:            tags,
		}
		post.Images, err = util.DecodeStringSlice(postRow.Images)
		if err != nil {
			return fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(postRow.Attachments)
		if err != nil {
			return fmt.Errorf("decode attachments: %w", err)
		}

		resp = &api.PostCounterResponse{Post: post}
		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PostService) CollectPost(ctx context.Context, uid string, req *api.CollectPostRequest) (*api.PostCounterResponse, error) {
	var resp *api.PostCounterResponse

	if err := db.WithTx(ctx, s.dbx, s.db, func(qtx *db.Queries) error {
		postRow, err := qtx.GetPostByUid(ctx, req.Uid)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("post not found")
			}
			return fmt.Errorf("get post: %w", err)
		}

		var delta int64
		if req.Action == api.ToggleAction_TOGGLE_ACTION_REMOVE {
			affected, err := qtx.DeletePostCollection(ctx, db.DeletePostCollectionParams{
				PostUid: req.Uid,
				UserUid: uid,
			})
			if err != nil {
				return fmt.Errorf("delete collection: %w", err)
			}
			delta = -affected
		} else {
			affected, err := qtx.InsertPostCollection(ctx, db.InsertPostCollectionParams{
				PostUid: req.Uid,
				UserUid: uid,
			})
			if err != nil {
				return fmt.Errorf("insert collection: %w", err)
			}
			delta = affected
		}

		if delta != 0 {
			if _, err := qtx.UpdatePostCollectionCount(ctx, db.UpdatePostCollectionCountParams{
				Delta: delta,
				Uid:   req.Uid,
			}); err != nil {
				return fmt.Errorf("update collection count: %w", err)
			}
			postRow.CollectionCount += delta
			if postRow.CollectionCount < 0 {
				postRow.CollectionCount = 0
			}
		}

		tags, err := qtx.ListPostTagsByUid(ctx, req.Uid)
		if err != nil {
			return fmt.Errorf("list tags: %w", err)
		}

		post := &api.Post{
			Uid:             postRow.Uid,
			Author:          s.postAuthor(postRow.Author, postRow.AuthorNickname, postRow.AuthorAvatarUrl),
			Text:            postRow.Text,
			CommentCount:    postRow.CommentCount,
			CollectionCount: postRow.CollectionCount,
			LikeCount:       postRow.LikeCount,
			Visibility:      postRow.Visibility,
			LatestRepliedOn: postRow.LatestRepliedOn,
			Ip:              postRow.Ip,
			Pinned:          postRow.Pinned == 1,
			CreatedAt:       postRow.CreatedAt,
			UpdatedAt:       postRow.UpdatedAt,
			Tags:            tags,
		}

		post.Images, err = util.DecodeStringSlice(postRow.Images)
		if err != nil {
			return fmt.Errorf("decode images: %w", err)
		}
		post.Attachments, err = util.DecodeStringSlice(postRow.Attachments)
		if err != nil {
			return fmt.Errorf("decode attachments: %w", err)
		}

		resp = &api.PostCounterResponse{Post: post}
		return nil
	}); err != nil {
		return nil, err
	}

	return resp, nil
}

func (s *PostService) postAuthor(uid, nickname, avatarURL string) *api.PostAuthor {
	return &api.PostAuthor{
		Uid:       uid,
		Nickname:  nickname,
		AvatarUrl: avatarURL,
	}
}

func (s *PostService) nsPtr(p *string) sql.NullString {
	if p == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *p, Valid: true}
}
