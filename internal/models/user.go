package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID          uuid.UUID  `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Username    string     `json:"username" gorm:"uniqueIndex;not null"`
	Email       string     `json:"email" gorm:"uniqueIndex;not null"`
	Password    string     `json:"-" gorm:"not null"`
	DisplayName string     `json:"display_name"`
	Avatar      string     `json:"avatar"`
	Bio         string     `json:"bio"`
	Followers   int64      `json:"followers" gorm:"default:0"`
	Following   int64      `json:"following" gorm:"default:0"`
	IsActive    bool       `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type Follow struct {
	ID          uuid.UUID `json:"id" gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	FollowerID  uuid.UUID `json:"follower_id" gorm:"type:uuid;not null;index:idx_follower_following"`
	FollowingID uuid.UUID `json:"following_id" gorm:"type:uuid;not null;index:idx_follower_following"`
	CreatedAt   time.Time `json:"created_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`

	Follower  User `json:"follower" gorm:"foreignKey:FollowerID"`
	Following User `json:"following" gorm:"foreignKey:FollowingID"`
}

func (User) TableName() string {
	return "users"
}

func (Follow) TableName() string {
	return "follows"
}