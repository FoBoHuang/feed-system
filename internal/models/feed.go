package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Post struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID      uuid.UUID  `json:"user_id" gorm:"type:uuid;not null;index"`
	Content     string     `json:"content" gorm:"type:text;not null"`
	ImageURLs   []string   `json:"image_urls" gorm:"type:text[]"`
	LikeCount   int64      `json:"like_count" gorm:"default:0"`
	CommentCount int64     `json:"comment_count" gorm:"default:0"`
	ShareCount  int64      `json:"share_count" gorm:"default:0"`
	Score       float64    `json:"score" gorm:"default:0"` // 用于排序的分数
	IsDeleted   bool       `json:"is_deleted" gorm:"default:false"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	User User `json:"user" gorm:"foreignKey:UserID"`
}

type Like struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index:idx_user_post"`
	PostID    uuid.UUID `json:"post_id" gorm:"type:uuid;not null;index:idx_user_post"`
	CreatedAt time.Time `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	User User `json:"user" gorm:"foreignKey:UserID"`
	Post Post `json:"post" gorm:"foreignKey:PostID"`
}

type Comment struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null"`
	PostID    uuid.UUID `json:"post_id" gorm:"type:uuid;not null;index"`
	Content   string    `json:"content" gorm:"type:text;not null"`
	ParentID  *uuid.UUID `json:"parent_id" gorm:"type:uuid"`
	LikeCount int64     `json:"like_count" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	User User `json:"user" gorm:"foreignKey:UserID"`
	Post Post `json:"post" gorm:"foreignKey:PostID"`
}

type Timeline struct {
	ID        uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index:idx_user_post"`
	PostID    uuid.UUID `json:"post_id" gorm:"type:uuid;not null;index:idx_user_post"`
	Score     float64   `json:"score" gorm:"default:0"`
	CreatedAt time.Time `json:"created_at" gorm:"index"`

	User User `json:"user" gorm:"foreignKey:UserID"`
	Post Post `json:"post" gorm:"foreignKey:PostID"`
}

func (Post) TableName() string {
	return "posts"
}

func (Like) TableName() string {
	return "likes"
}

func (Comment) TableName() string {
	return "comments"
}

func (Timeline) TableName() string {
	return "timelines"
}