package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/feed-system/feed-system/internal/models"
	"github.com/feed-system/feed-system/internal/repository"
	"github.com/feed-system/feed-system/pkg/logger"
	"github.com/feed-system/feed-system/pkg/queue"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	userRepo   *repository.UserRepository
	followRepo *repository.FollowRepository
	producer   *queue.KafkaProducer
	logger     *logger.Logger
}

func NewUserService(userRepo *repository.UserRepository, followRepo *repository.FollowRepository, producer *queue.KafkaProducer, logger *logger.Logger) *UserService {
	return &UserService{
		userRepo:   userRepo,
		followRepo: followRepo,
		producer:   producer,
		logger:     logger,
	}
}

type RegisterRequest struct {
	Username    string `json:"username" binding:"required,min=3,max=30"`
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=6,max=50"`
	DisplayName string `json:"display_name" binding:"max=50"`
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type UpdateUserRequest struct {
	DisplayName *string `json:"display_name" binding:"max=50"`
	Avatar      *string `json:"avatar"`
	Bio         *string `json:"bio" binding:"max=500"`
}

type FollowRequest struct {
	UserID     string `json:"user_id" binding:"required"`
	FollowingID string `json:"following_id" binding:"required"`
}

func (s *UserService) Register(ctx context.Context, req *RegisterRequest) (*models.User, error) {
	// 检查用户名是否已存在
	existingUser, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to check username: %w", err)
	}
	if existingUser != nil {
		return nil, errors.New("username already exists")
	}

	// 检查邮箱是否已存在
	existingUser, err = s.userRepo.GetByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to check email: %w", err)
	}
	if existingUser != nil {
		return nil, errors.New("email already exists")
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// 创建用户
	user := &models.User{
		Username:    req.Username,
		Email:       req.Email,
		Password:    string(hashedPassword),
		DisplayName: req.DisplayName,
		IsActive:    true,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// 发送用户创建事件
	event := queue.Event{
		Type:      queue.EventUserCreated,
		Timestamp: user.CreatedAt,
		Data: map[string]interface{}{
			"user_id":      user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
		},
	}
	if err := s.producer.Publish(ctx, user.ID.String(), event); err != nil {
		s.logger.WithError(err).Error("Failed to publish user created event")
	}

	s.logger.WithField("user_id", user.ID).Info("User registered successfully")
	return user, nil
}

func (s *UserService) Login(ctx context.Context, req *LoginRequest) (*models.User, error) {
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("invalid username or password")
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, errors.New("invalid username or password")
	}

	if !user.IsActive {
		return nil, errors.New("user account is inactive")
	}

	s.logger.WithField("user_id", user.ID).Info("User logged in successfully")
	return user, nil
}

func (s *UserService) GetByID(ctx context.Context, userID string) (*models.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

func (s *UserService) Update(ctx context.Context, userID string, req *UpdateUserRequest) (*models.User, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	user, err := s.userRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// 更新字段
	if req.DisplayName != nil {
		user.DisplayName = *req.DisplayName
	}
	if req.Avatar != nil {
		user.Avatar = *req.Avatar
	}
	if req.Bio != nil {
		user.Bio = *req.Bio
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// 发送用户更新事件
	event := queue.Event{
		Type:      queue.EventUserUpdated,
		Timestamp: user.UpdatedAt,
		Data: map[string]interface{}{
			"user_id":      user.ID,
			"display_name": user.DisplayName,
			"avatar":       user.Avatar,
			"bio":          user.Bio,
		},
	}
	if err := s.producer.Publish(ctx, user.ID.String(), event); err != nil {
		s.logger.WithError(err).Error("Failed to publish user updated event")
	}

	s.logger.WithField("user_id", user.ID).Info("User updated successfully")
	return user, nil
}

func (s *UserService) Follow(ctx context.Context, followerID, followingID string) error {
	followerUUID, err := uuid.Parse(followerID)
	if err != nil {
		return fmt.Errorf("invalid follower ID: %w", err)
	}

	followingUUID, err := uuid.Parse(followingID)
	if err != nil {
		return fmt.Errorf("invalid following ID: %w", err)
	}

	// 检查用户是否存在
	follower, err := s.userRepo.GetByID(ctx, followerUUID)
	if err != nil {
		return fmt.Errorf("failed to get follower: %w", err)
	}
	if follower == nil {
		return errors.New("follower not found")
	}

	following, err := s.userRepo.GetByID(ctx, followingUUID)
	if err != nil {
		return fmt.Errorf("failed to get following: %w", err)
	}
	if following == nil {
		return errors.New("following user not found")
	}

	// 检查是否已经关注
	existingFollow, err := s.followRepo.Get(ctx, followerUUID, followingUUID)
	if err != nil {
		return fmt.Errorf("failed to check follow status: %w", err)
	}
	if existingFollow != nil {
		return errors.New("already following")
	}

	// 创建关注关系
	follow := &models.Follow{
		FollowerID:  followerUUID,
		FollowingID: followingUUID,
	}

	if err := s.followRepo.Create(ctx, follow); err != nil {
		return fmt.Errorf("failed to create follow: %w", err)
	}

	// 更新关注数和粉丝数
	if err := s.userRepo.UpdateFollowingCount(ctx, followerUUID, 1); err != nil {
		s.logger.WithError(err).Error("Failed to update following count")
	}
	if err := s.userRepo.UpdateFollowersCount(ctx, followingUUID, 1); err != nil {
		s.logger.WithError(err).Error("Failed to update followers count")
	}

	// 发送关注事件
	event := queue.Event{
		Type:      queue.EventFollowCreated,
		Timestamp: follow.CreatedAt,
		Data: queue.FollowEventData{
			FollowerID:  followerID,
			FollowingID: followingID,
			CreatedAt:   follow.CreatedAt.Format("2006-01-02T15:04:05Z"),
		},
	}
	if err := s.producer.Publish(ctx, followerID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish follow created event")
	}

	s.logger.WithFields(map[string]interface{}{
		"follower_id":  followerID,
		"following_id": followingID,
	}).Info("User followed successfully")

	return nil
}

func (s *UserService) Unfollow(ctx context.Context, followerID, followingID string) error {
	followerUUID, err := uuid.Parse(followerID)
	if err != nil {
		return fmt.Errorf("invalid follower ID: %w", err)
	}

	followingUUID, err := uuid.Parse(followingID)
	if err != nil {
		return fmt.Errorf("invalid following ID: %w", err)
	}

	// 检查关注关系是否存在
	existingFollow, err := s.followRepo.Get(ctx, followerUUID, followingUUID)
	if err != nil {
		return fmt.Errorf("failed to check follow status: %w", err)
	}
	if existingFollow == nil {
		return errors.New("not following")
	}

	// 删除关注关系
	if err := s.followRepo.Delete(ctx, followerUUID, followingUUID); err != nil {
		return fmt.Errorf("failed to delete follow: %w", err)
	}

	// 更新关注数和粉丝数
	if err := s.userRepo.UpdateFollowingCount(ctx, followerUUID, -1); err != nil {
		s.logger.WithError(err).Error("Failed to update following count")
	}
	if err := s.userRepo.UpdateFollowersCount(ctx, followingUUID, -1); err != nil {
		s.logger.WithError(err).Error("Failed to update followers count")
	}

	// 发送取消关注事件
	event := queue.Event{
		Type:      queue.EventFollowDeleted,
		Timestamp: existingFollow.CreatedAt,
		Data: queue.FollowEventData{
			FollowerID:  followerID,
			FollowingID: followingID,
		},
	}
	if err := s.producer.Publish(ctx, followerID, event); err != nil {
		s.logger.WithError(err).Error("Failed to publish follow deleted event")
	}

	s.logger.WithFields(map[string]interface{}{
		"follower_id":  followerID,
		"following_id": followingID,
	}).Info("User unfollowed successfully")

	return nil
}

func (s *UserService) GetFollowers(ctx context.Context, userID string, offset, limit int) ([]*models.User, error) {
	uuid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	followers, err := s.followRepo.GetFollowers(ctx, uuid, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get followers: %w", err)
	}

	return followers, nil
}

func (s *UserService) GetFollowing(ctx context.Context, userID string, offset, limit int) ([]*models.User, error) {
	uuid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	following, err := s.followRepo.GetFollowing(ctx, uuid, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get following: %w", err)
	}

	return following, nil
}

func (s *UserService) IsFollowing(ctx context.Context, followerID, followingID string) (bool, error) {
	followerUUID, err := uuid.Parse(followerID)
	if err != nil {
		return false, fmt.Errorf("invalid follower ID: %w", err)
	}

	followingUUID, err := uuid.Parse(followingID)
	if err != nil {
		return false, fmt.Errorf("invalid following ID: %w", err)
	}

	return s.followRepo.IsFollowing(ctx, followerUUID, followingUUID)
}

func (s *UserService) Search(ctx context.Context, query string, offset, limit int) ([]*models.User, error) {
	users, err := s.userRepo.Search(ctx, query, offset, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search users: %w", err)
	}
	return users, nil
}