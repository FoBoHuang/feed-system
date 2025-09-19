package handlers

import (
	"net/http"
	"strconv"

	"github.com/feed-system/feed-system/internal/middleware"
	"github.com/feed-system/feed-system/internal/services"
	"github.com/gin-gonic/gin"
)

type FeedHandler struct {
	feedService    *services.FeedService
	likeService    *services.LikeService
	commentService *services.CommentService
}

func NewFeedHandler(feedService *services.FeedService, likeService *services.LikeService, commentService *services.CommentService) *FeedHandler {
	return &FeedHandler{
		feedService:    feedService,
		likeService:    likeService,
		commentService: commentService,
	}
}

func (h *FeedHandler) CreatePost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var req services.CreatePostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	post, err := h.feedService.CreatePost(c.Request.Context(), userID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Post created successfully",
		"post":    post,
	})
}

func (h *FeedHandler) GetFeed(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	cursor := c.Query("cursor")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil && parsedLimit > 0 && parsedLimit <= 100 {
			limit = parsedLimit
		}
	}

	feed, err := h.feedService.GetFeed(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, feed)
}

func (h *FeedHandler) GetUserPosts(c *gin.Context) {
	targetUserID := c.Param("id")
	if targetUserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User ID is required"})
		return
	}

	offset := 0
	limit := 20
	query := struct {
		Offset int `form:"offset"`
		Limit  int `form:"limit"`
	}{}
	if err := c.ShouldBindQuery(&query); err == nil {
		offset = query.Offset
		limit = query.Limit
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	posts, err := h.feedService.GetUserPosts(c.Request.Context(), targetUserID, offset, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"posts":  posts,
		"offset": offset,
		"limit":  limit,
	})
}

func (h *FeedHandler) GetPost(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	post, err := h.feedService.GetPostByID(c.Request.Context(), postID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"post": post})
}

func (h *FeedHandler) DeletePost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	if err := h.feedService.DeletePost(c.Request.Context(), userID, postID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post deleted successfully"})
}

func (h *FeedHandler) LikePost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	if err := h.likeService.LikePost(c.Request.Context(), userID, postID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post liked successfully"})
}

func (h *FeedHandler) UnlikePost(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	if err := h.likeService.UnlikePost(c.Request.Context(), userID, postID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Post unliked successfully"})
}

func (h *FeedHandler) GetPostLikes(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	offset := 0
	limit := 20
	query := struct {
		Offset int `form:"offset"`
		Limit  int `form:"limit"`
	}{}
	if err := c.ShouldBindQuery(&query); err == nil {
		offset = query.Offset
		limit = query.Limit
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	likes, err := h.likeService.GetPostLikes(c.Request.Context(), postID, offset, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"likes":  likes,
		"offset": offset,
		"limit":  limit,
	})
}

func (h *FeedHandler) CreateComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	var req services.CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	comment, err := h.commentService.CreateComment(c.Request.Context(), userID, postID, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Comment created successfully",
		"comment": comment,
	})
}

func (h *FeedHandler) GetPostComments(c *gin.Context) {
	postID := c.Param("id")
	if postID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Post ID is required"})
		return
	}

	offset := 0
	limit := 20
	query := struct {
		Offset int `form:"offset"`
		Limit  int `form:"limit"`
	}{}
	if err := c.ShouldBindQuery(&query); err == nil {
		offset = query.Offset
		limit = query.Limit
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	comments, err := h.commentService.GetPostComments(c.Request.Context(), postID, offset, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"comments": comments,
		"offset":   offset,
		"limit":    limit,
	})
}

func (h *FeedHandler) DeleteComment(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	commentID := c.Param("id")
	if commentID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Comment ID is required"})
		return
	}

	if err := h.commentService.DeleteComment(c.Request.Context(), userID, commentID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Comment deleted successfully"})
}

func (h *FeedHandler) SearchPosts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query is required"})
		return
	}

	offset := 0
	limit := 20
	queryParams := struct {
		Query  string `form:"q"`
		Offset int    `form:"offset"`
		Limit  int    `form:"limit"`
	}{}
	if err := c.ShouldBindQuery(&queryParams); err == nil {
		query = queryParams.Query
		offset = queryParams.Offset
		limit = queryParams.Limit
		if limit > 100 {
			limit = 100
		}
		if limit < 1 {
			limit = 1
		}
	}

	posts, err := h.feedService.SearchPosts(c.Request.Context(), query, offset, limit)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"posts":  posts,
		"query":  query,
		"offset": offset,
		"limit":  limit,
	})
}