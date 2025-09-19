# Feed流系统 - API接口文档

## 接口概览

### 基础信息
- **Base URL**: `http://localhost:8080/api/v1`
- **认证方式**: JWT Bearer Token
- **数据格式**: JSON
- **字符编码**: UTF-8

### 通用响应格式
```json
{
  "code": 200,
  "message": "success",
  "data": {},
  "timestamp": 1640995200000
}
```

### 通用错误格式
```json
{
  "code": 400,
  "error": "错误信息",
  "timestamp": 1640995200000
}
```

### 状态码说明
- `200`: 请求成功
- `201`: 创建成功
- `400`: 请求参数错误
- `401`: 未认证
- `403`: 无权限
- `404`: 资源不存在
- `500`: 服务器内部错误

## 认证接口

### 1. 用户注册

**POST** `/users/register`

**请求参数：**
```json
{
  "username": "testuser",
  "email": "test@example.com",
  "password": "password123",
  "display_name": "测试用户"
}
```

**参数说明：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| username | string | 是 | 用户名，3-30字符 |
| email | string | 是 | 邮箱地址 |
| password | string | 是 | 密码，6-50字符 |
| display_name | string | 否 | 显示名称，最大50字符 |

**响应示例：**
```json
{
  "code": 201,
  "message": "User registered successfully",
  "data": {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "testuser",
      "email": "test@example.com",
      "display_name": "测试用户",
      "avatar": "",
      "bio": "",
      "followers": 0,
      "following": 0,
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z"
    }
  },
  "timestamp": 1640995200000
}
```

**错误响应：**
```json
{
  "code": 400,
  "error": "Username already exists",
  "timestamp": 1640995200000
}
```

### 2. 用户登录

**POST** `/users/login`

**请求参数：**
```json
{
  "username": "testuser",
  "password": "password123"
}
```

**响应示例：**
```json
{
  "code": 200,
  "message": "Login successful",
  "data": {
    "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "testuser",
      "email": "test@example.com",
      "display_name": "测试用户",
      "avatar": "",
      "bio": "",
      "followers": 0,
      "following": 0,
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z"
    }
  },
  "timestamp": 1640995200000
}
```

## 用户管理接口

### 3. 获取用户信息

**GET** `/users/{id}`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 用户ID |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "testuser",
      "email": "test@example.com",
      "display_name": "测试用户",
      "avatar": "https://example.com/avatar.jpg",
      "bio": "这是我的个人简介",
      "followers": 100,
      "following": 50,
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-02T00:00:00Z"
    }
  },
  "timestamp": 1640995200000
}
```

### 4. 更新用户资料

**PUT** `/users/profile`

**请求头：**
```
Authorization: Bearer {token}
Content-Type: application/json
```

**请求参数：**
```json
{
  "display_name": "新显示名称",
  "avatar": "https://example.com/new-avatar.jpg",
  "bio": "更新后的个人简介"
}
```

**响应示例：**
```json
{
  "code": 200,
  "message": "Profile updated successfully",
  "data": {
    "user": {
      "id": "123e4567-e89b-12d3-a456-426614174000",
      "username": "testuser",
      "email": "test@example.com",
      "display_name": "新显示名称",
      "avatar": "https://example.com/new-avatar.jpg",
      "bio": "更新后的个人简介",
      "followers": 100,
      "following": 50,
      "is_active": true,
      "created_at": "2024-01-01T00:00:00Z",
      "updated_at": "2024-01-02T00:00:00Z"
    }
  },
  "timestamp": 1640995200000
}
```

### 5. 关注用户

**POST** `/users/follow`

**请求头：**
```
Authorization: Bearer {token}
Content-Type: application/json
```

**请求参数：**
```json
{
  "user_id": "123e4567-e89b-12d3-a456-426614174000",
  "following_id": "987fcdeb-51a2-43d2-b456-426614174000"
}
```

**响应示例：**
```json
{
  "code": 200,
  "message": "Followed successfully",
  "timestamp": 1640995200000
}
```

### 6. 取消关注

**DELETE** `/users/unfollow/{id}`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 要取消关注的用户ID |

**响应示例：**
```json
{
  "code": 200,
  "message": "Unfollowed successfully",
  "timestamp": 1640995200000
}
```

### 7. 获取关注列表

**GET** `/users/{id}/following`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 用户ID |

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| offset | integer | 否 | 偏移量，默认0 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "following": [
      {
        "id": "987fcdeb-51a2-43d2-b456-426614174000",
        "username": "followeduser",
        "display_name": "被关注用户",
        "avatar": "https://example.com/avatar.jpg",
        "bio": "个人简介",
        "followers": 200,
        "following": 150,
        "is_active": true,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

### 8. 获取粉丝列表

**GET** `/users/{id}/followers`

**请求头：**
```
Authorization: Bearer {token}
```

**参数说明：** 同获取关注列表

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "followers": [
      {
        "id": "456def78-9abc-def0-1234-567890abcdef",
        "username": "followeruser",
        "display_name": "粉丝用户",
        "avatar": "https://example.com/avatar.jpg",
        "bio": "个人简介",
        "followers": 50,
        "following": 80,
        "is_active": true,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

### 9. 搜索用户

**GET** `/users/search`

**请求头：**
```
Authorization: Bearer {token}
```

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| q | string | 是 | 搜索关键词 |
| offset | integer | 否 | 偏移量，默认0 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "users": [
      {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "username": "testuser",
        "display_name": "测试用户",
        "avatar": "https://example.com/avatar.jpg",
        "bio": "个人简介",
        "followers": 100,
        "following": 50,
        "is_active": true,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "query": "test",
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

## Feed接口

### 10. 创建帖子

**POST** `/posts`

**请求头：**
```
Authorization: Bearer {token}
Content-Type: application/json
```

**请求参数：**
```json
{
  "content": "这是我的第一条动态！",
  "image_urls": [
    "https://example.com/image1.jpg",
    "https://example.com/image2.jpg"
  ]
}
```

**参数说明：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| content | string | 是 | 帖子内容，1-1000字符 |
| image_urls | array[string] | 否 | 图片URL数组，最多9张 |

**响应示例：**
```json
{
  "code": 201,
  "message": "Post created successfully",
  "data": {
    "post": {
      "id": "abcdef12-3456-7890-abcd-ef1234567890",
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "content": "这是我的第一条动态！",
      "image_urls": [
        "https://example.com/image1.jpg",
        "https://example.com/image2.jpg"
      ],
      "like_count": 0,
      "comment_count": 0,
      "share_count": 0,
      "score": 1.5,
      "is_deleted": false,
      "created_at": "2024-01-01T12:00:00Z",
      "updated_at": "2024-01-01T12:00:00Z",
      "user": {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "username": "testuser",
        "display_name": "测试用户",
        "avatar": "https://example.com/avatar.jpg"
      }
    }
  },
  "timestamp": 1640995200000
}
```

### 11. 获取Feed流

**GET** `/feed`

**请求头：**
```
Authorization: Bearer {token}
```

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| cursor | string | 否 | 分页游标，用于获取下一页 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "posts": [
      {
        "id": "abcdef12-3456-7890-abcd-ef1234567890",
        "user_id": "123e4567-e89b-12d3-a456-426614174000",
        "content": "这是我的第一条动态！",
        "image_urls": ["https://example.com/image1.jpg"],
        "like_count": 10,
        "comment_count": 5,
        "share_count": 2,
        "score": 1.8,
        "is_deleted": false,
        "created_at": "2024-01-01T12:00:00Z",
        "user": {
          "id": "123e4567-e89b-12d3-a456-426614174000",
          "username": "testuser",
          "display_name": "测试用户",
          "avatar": "https://example.com/avatar.jpg"
        }
      }
    ],
    "next_cursor": "eyJvZmZzZXQiOjIwfQ==",
    "has_more": true
  },
  "timestamp": 1640995200000
}
```

### 12. 获取用户帖子

**GET** `/users/{id}/posts`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 用户ID |

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| offset | integer | 否 | 偏移量，默认0 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "posts": [
      {
        "id": "abcdef12-3456-7890-abcd-ef1234567890",
        "user_id": "123e4567-e89b-12d3-a456-426614174000",
        "content": "这是我的第一条动态！",
        "image_urls": ["https://example.com/image1.jpg"],
        "like_count": 10,
        "comment_count": 5,
        "share_count": 2,
        "score": 1.8,
        "is_deleted": false,
        "created_at": "2024-01-01T12:00:00Z",
        "user": {
          "id": "123e4567-e89b-12d3-a456-426614174000",
          "username": "testuser",
          "display_name": "测试用户",
          "avatar": "https://example.com/avatar.jpg"
        }
      }
    ],
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

### 13. 获取帖子详情

**GET** `/posts/{id}`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "post": {
      "id": "abcdef12-3456-7890-abcd-ef1234567890",
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "content": "这是我的第一条动态！",
      "image_urls": ["https://example.com/image1.jpg"],
      "like_count": 10,
      "comment_count": 5,
      "share_count": 2,
      "score": 1.8,
      "is_deleted": false,
      "created_at": "2024-01-01T12:00:00Z",
      "updated_at": "2024-01-01T12:00:00Z",
      "user": {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "username": "testuser",
        "display_name": "测试用户",
        "avatar": "https://example.com/avatar.jpg"
      }
    }
  },
  "timestamp": 1640995200000
}
```

### 14. 删除帖子

**DELETE** `/posts/{id}`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**响应示例：**
```json
{
  "code": 200,
  "message": "Post deleted successfully",
  "timestamp": 1640995200000
}
```

### 15. 搜索帖子

**GET** `/posts/search`

**请求头：**
```
Authorization: Bearer {token}
```

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| q | string | 是 | 搜索关键词 |
| offset | integer | 否 | 偏移量，默认0 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "posts": [
      {
        "id": "abcdef12-3456-7890-abcd-ef1234567890",
        "user_id": "123e4567-e89b-12d3-a456-426614174000",
        "content": "搜索关键词出现在这里",
        "image_urls": ["https://example.com/image1.jpg"],
        "like_count": 10,
        "comment_count": 5,
        "share_count": 2,
        "score": 1.8,
        "is_deleted": false,
        "created_at": "2024-01-01T12:00:00Z",
        "user": {
          "id": "123e4567-e89b-12d3-a456-426614174000",
          "username": "testuser",
          "display_name": "测试用户",
          "avatar": "https://example.com/avatar.jpg"
        }
      }
    ],
    "query": "搜索关键词",
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

## 互动接口

### 16. 点赞帖子

**POST** `/posts/{id}/like`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**响应示例：**
```json
{
  "code": 200,
  "message": "Post liked successfully",
  "timestamp": 1640995200000
}
```

### 17. 取消点赞

**DELETE** `/posts/{id}/like`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**响应示例：**
```json
{
  "code": 200,
  "message": "Post unliked successfully",
  "timestamp": 1640995200000
}
```

### 18. 获取帖子点赞列表

**GET** `/posts/{id}/likes`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| offset | integer | 否 | 偏移量，默认0 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "likes": [
      {
        "id": "like-123e4567-e89b-12d3-a456-426614174000",
        "user_id": "123e4567-e89b-12d3-a456-426614174000",
        "post_id": "abcdef12-3456-7890-abcd-ef1234567890",
        "created_at": "2024-01-01T12:30:00Z",
        "user": {
          "id": "123e4567-e89b-12d3-a456-426614174000",
          "username": "testuser",
          "display_name": "测试用户",
          "avatar": "https://example.com/avatar.jpg"
        }
      }
    ],
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

### 19. 创建评论

**POST** `/posts/{id}/comments`

**请求头：**
```
Authorization: Bearer {token}
Content-Type: application/json
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**请求参数：**
```json
{
  "content": "这是一条评论",
  "parent_id": null
}
```

**参数说明：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| content | string | 是 | 评论内容，1-500字符 |
| parent_id | string | 否 | 父评论ID，用于回复 |

**响应示例：**
```json
{
  "code": 201,
  "message": "Comment created successfully",
  "data": {
    "comment": {
      "id": "comment-123e4567-e89b-12d3-a456-426614174000",
      "user_id": "123e4567-e89b-12d3-a456-426614174000",
      "post_id": "abcdef12-3456-7890-abcd-ef1234567890",
      "content": "这是一条评论",
      "parent_id": null,
      "like_count": 0,
      "created_at": "2024-01-01T12:30:00Z",
      "user": {
        "id": "123e4567-e89b-12d3-a456-426614174000",
        "username": "testuser",
        "display_name": "测试用户",
        "avatar": "https://example.com/avatar.jpg"
      }
    }
  },
  "timestamp": 1640995200000
}
```

### 20. 获取评论列表

**GET** `/posts/{id}/comments`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 帖子ID |

**查询参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| offset | integer | 否 | 偏移量，默认0 |
| limit | integer | 否 | 返回数量，默认20，最大100 |

**响应示例：**
```json
{
  "code": 200,
  "data": {
    "comments": [
      {
        "id": "comment-123e4567-e89b-12d3-a456-426614174000",
        "user_id": "123e4567-e89b-12d3-a456-426614174000",
        "post_id": "abcdef12-3456-7890-abcd-ef1234567890",
        "content": "这是一条评论",
        "parent_id": null,
        "like_count": 5,
        "created_at": "2024-01-01T12:30:00Z",
        "user": {
          "id": "123e4567-e89b-12d3-a456-426614174000",
          "username": "testuser",
          "display_name": "测试用户",
          "avatar": "https://example.com/avatar.jpg"
        }
      }
    ],
    "offset": 0,
    "limit": 20
  },
  "timestamp": 1640995200000
}
```

### 21. 删除评论

**DELETE** `/comments/{id}`

**请求头：**
```
Authorization: Bearer {token}
```

**路径参数：**
| 参数 | 类型 | 必需 | 描述 |
|------|------|------|------|
| id | string | 是 | 评论ID |

**响应示例：**
```json
{
  "code": 200,
  "message": "Comment deleted successfully",
  "timestamp": 1640995200000
}
```

## 分页机制

### 游标分页
Feed流接口使用游标分页机制，避免深度分页的性能问题。

**游标格式：**
```
cursor: eyJvZmZzZXQiOjIwfQ==  // base64编码的JSON: {"offset": 20}
```

**使用方式：**
1. 首次请求不携带cursor参数
2. 从响应中获取next_cursor
3. 下次请求携带cursor参数获取下一页
4. 当has_more为false时，表示没有更多数据

**示例：**
```bash
# 第一页
curl -H "Authorization: Bearer {token}" \
  "http://localhost:8080/api/v1/feed?limit=20"

# 第二页（使用返回的next_cursor）
curl -H "Authorization: Bearer {token}" \
  "http://localhost:8080/api/v1/feed?cursor=eyJvZmZzZXQiOjIwfQ==&limit=20"
```

## 错误处理

### 错误码说明
| 错误码 | 描述 | 示例 |
|--------|------|------|
| 400 | 请求参数错误 | 缺少必需参数、参数格式错误 |
| 401 | 未认证 | Token缺失或无效 |
| 403 | 无权限 | 尝试删除他人帖子 |
| 404 | 资源不存在 | 用户、帖子不存在 |
| 409 | 资源冲突 | 重复关注、重复点赞 |
| 422 | 业务逻辑错误 | 不能关注自己 |
| 429 | 请求频率限制 | 请求过于频繁 |
| 500 | 服务器内部错误 | 数据库连接失败等 |

### 错误响应格式
```json
{
  "code": 400,
  "error": "具体错误信息",
  "details": {
    "field": "username",
    "message": "Username already exists"
  },
  "timestamp": 1640995200000
}
```

## 限流说明

### 限流规则
- 未认证请求：100次/分钟/IP
- 认证请求：1000次/分钟/用户
- 特殊接口（如注册）：10次/分钟/IP

### 限流响应
当触发限流时，返回429状态码：
```json
{
  "code": 429,
  "error": "Too many requests",
  "retry_after": 60,
  "timestamp": 1640995200000
}
```

## 版本管理

### 版本策略
- URL版本控制：/api/v1/
- 向后兼容：新版本保持旧版本接口兼容
- 版本弃用：提前通知版本弃用计划

### 版本升级
- v1 → v2：重大功能变更
- v1.1 → v1.2：小功能增强
- v1.1.0 → v1.1.1：Bug修复

## 测试工具

### cURL示例
```bash
# 用户注册
curl -X POST http://localhost:8080/api/v1/users/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "password123",
    "display_name": "测试用户"
  }'

# 用户登录
curl -X POST http://localhost:8080/api/v1/users/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "password": "password123"
  }'

# 创建帖子
curl -X POST http://localhost:8080/api/v1/posts \
  -H "Authorization: Bearer YOUR_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Hello World!",
    "image_urls": []
  }'

# 获取Feed
curl -X GET "http://localhost:8080/api/v1/feed?limit=20" \
  -H "Authorization: Bearer YOUR_TOKEN"
```

### Postman集合
提供了完整的Postman API集合文件：`docs/postman/FeedSystem.postman_collection.json`

### API测试脚本
```bash
# 运行API测试
./scripts/test-api.sh

# 性能测试
./scripts/benchmark.sh
```

## SDK和工具

### 官方SDK
- **Go SDK**: github.com/feed-system/go-sdk
- **JavaScript SDK**: github.com/feed-system/js-sdk
- **Python SDK**: github.com/feed-system/python-sdk

### 第三方工具
- **OpenAPI文档**: http://localhost:8080/swagger/
- **API测试工具**: 集成Swagger UI
- **性能监控**: Prometheus + Grafana

## 更新日志

### v1.0.0 (2024-01-01)
- 初始版本发布
- 用户注册登录
- Feed流功能
- 点赞评论功能
- 基础搜索功能

### v1.1.0 (2024-02-01)
- 添加图片上传支持
- 优化Feed排序算法
- 增加批量操作接口
- 性能优化

---

这份API文档提供了Feed流系统的完整接口说明，包括请求参数、响应格式、错误处理等详细信息，方便开发者快速集成和使用。文档会持续更新，建议关注版本变化。