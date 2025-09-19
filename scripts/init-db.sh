#!/bin/bash

# Feed流系统数据库初始化脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DB_NAME="${DB_NAME:-feedsystem}"
DB_USER="${DB_USER:-feeduser}"
DB_PASSWORD="${DB_PASSWORD:-feedpass}"

# 输出函数
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# 检查PostgreSQL连接
check_postgres_connection() {
    print_info "检查PostgreSQL连接..."

    if ! command -v psql > /dev/null 2>&1; then
        print_error "psql命令未找到，请先安装PostgreSQL客户端"
        exit 1
    fi

    # 测试连接
    if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "SELECT 1;" > /dev/null 2>&1; then
        print_success "PostgreSQL连接成功"
    else
        print_error "无法连接到PostgreSQL，请检查连接参数:"
        echo "  Host: $DB_HOST"
        echo "  Port: $DB_PORT"
        echo "  User: $DB_USER"
        echo "  Database: postgres"
        exit 1
    fi
}

# 创建数据库
create_database() {
    print_info "检查数据库是否存在..."

    # 检查数据库是否已存在
    if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -tAc "SELECT 1 FROM pg_database WHERE datname='$DB_NAME';" | grep -q 1; then
        print_warning "数据库 $DB_NAME 已存在"
        read -p "是否删除并重新创建数据库？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            print_info "删除现有数据库..."
            PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;"
            print_info "创建新数据库..."
            PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE $DB_NAME;"
            print_success "数据库创建成功"
        else
            print_info "使用现有数据库"
        fi
    else
        print_info "创建数据库 $DB_NAME..."
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "CREATE DATABASE $DB_NAME;"
        print_success "数据库创建成功"
    fi
}

# 创建扩展
create_extensions() {
    print_info "创建数据库扩展..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 创建UUID扩展
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 创建全文搜索扩展
CREATE EXTENSION IF NOT EXISTS "pg_trgm";

-- 创建数组操作扩展
CREATE EXTENSION IF NOT EXISTS "intarray";
EOF

    print_success "数据库扩展创建成功"
}

# 创建用户表
create_users_table() {
    print_info "创建用户表..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 用户表
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    username VARCHAR(30) UNIQUE NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    password VARCHAR(255) NOT NULL,
    display_name VARCHAR(50),
    avatar TEXT,
    bio TEXT,
    followers BIGINT DEFAULT 0,
    following BIGINT DEFAULT 0,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 用户表索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users(deleted_at);
EOF

    print_success "用户表创建成功"
}

# 创建关注关系表
create_follows_table() {
    print_info "创建关注关系表..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 关注关系表
CREATE TABLE IF NOT EXISTS follows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    follower_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    following_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    UNIQUE(follower_id, following_id)
);

-- 关注关系表索引
CREATE INDEX IF NOT EXISTS idx_follows_follower ON follows(follower_id);
CREATE INDEX IF NOT EXISTS idx_follows_following ON follows(following_id);
CREATE INDEX IF NOT EXISTS idx_follows_created_at ON follows(created_at);
CREATE INDEX IF NOT EXISTS idx_follows_deleted_at ON follows(deleted_at);
EOF

    print_success "关注关系表创建成功"
}

# 创建帖子表
create_posts_table() {
    print_info "创建帖子表..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 帖子表
CREATE TABLE IF NOT EXISTS posts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    image_urls TEXT[],
    like_count BIGINT DEFAULT 0,
    comment_count BIGINT DEFAULT 0,
    share_count BIGINT DEFAULT 0,
    score DOUBLE PRECISION DEFAULT 0,
    is_deleted BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 帖子表索引
CREATE INDEX IF NOT EXISTS idx_posts_user_id ON posts(user_id);
CREATE INDEX IF NOT EXISTS idx_posts_created_at ON posts(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_posts_score ON posts(score DESC);
CREATE INDEX IF NOT EXISTS idx_posts_deleted_at ON posts(deleted_at);

-- 全文搜索索引
CREATE INDEX IF NOT EXISTS idx_posts_content_gin ON posts USING gin(to_tsvector('chinese', content));
EOF

    print_success "帖子表创建成功"
}

# 创建时间线表
create_timelines_table() {
    print_info "创建时间线表..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 时间线表
CREATE TABLE IF NOT EXISTS timelines (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    score DOUBLE PRECISION DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, post_id)
);

-- 时间线表索引
CREATE INDEX IF NOT EXISTS idx_timelines_user_id ON timelines(user_id);
CREATE INDEX IF NOT EXISTS idx_timelines_score ON timelines(score DESC);
CREATE INDEX IF NOT EXISTS idx_timelines_created_at ON timelines(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_timelines_user_post ON timelines(user_id, post_id);
EOF

    print_success "时间线表创建成功"
}

# 创建点赞表
create_likes_table() {
    print_info "创建点赞表..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 点赞表
CREATE TABLE IF NOT EXISTS likes (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, post_id)
);

-- 点赞表索引
CREATE INDEX IF NOT EXISTS idx_likes_user_id ON likes(user_id);
CREATE INDEX IF NOT EXISTS idx_likes_post_id ON likes(post_id);
CREATE INDEX IF NOT EXISTS idx_likes_created_at ON likes(created_at);
EOF

    print_success "点赞表创建成功"
}

# 创建评论表
create_comments_table() {
    print_info "创建评论表..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 评论表
CREATE TABLE IF NOT EXISTS comments (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    parent_id UUID REFERENCES comments(id) ON DELETE CASCADE,
    like_count BIGINT DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP
);

-- 评论表索引
CREATE INDEX IF NOT EXISTS idx_comments_user_id ON comments(user_id);
CREATE INDEX IF NOT EXISTS idx_comments_post_id ON comments(post_id);
CREATE INDEX IF NOT EXISTS idx_comments_parent_id ON comments(parent_id);
CREATE INDEX IF NOT EXISTS idx_comments_created_at ON comments(created_at);
CREATE INDEX IF NOT EXISTS idx_comments_deleted_at ON comments(deleted_at);
EOF

    print_success "评论表创建成功"
}

# 创建触发器函数
create_trigger_functions() {
    print_info "创建触发器函数..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 更新用户关注数触发器
CREATE OR REPLACE FUNCTION update_user_follow_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE users SET following = following + 1 WHERE id = NEW.follower_id;
        UPDATE users SET followers = followers + 1 WHERE id = NEW.following_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE users SET following = following - 1 WHERE id = OLD.follower_id;
        UPDATE users SET followers = followers - 1 WHERE id = OLD.following_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- 更新帖子统计触发器
CREATE OR REPLACE FUNCTION update_post_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        IF NEW.parent_id IS NULL THEN
            UPDATE posts SET comment_count = comment_count + 1 WHERE id = NEW.post_id;
        END IF;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        IF OLD.parent_id IS NULL THEN
            UPDATE posts SET comment_count = comment_count - 1 WHERE id = OLD.post_id;
        END IF;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- 更新点赞统计触发器
CREATE OR REPLACE FUNCTION update_like_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        UPDATE posts SET like_count = like_count + 1 WHERE id = NEW.post_id;
        RETURN NEW;
    ELSIF TG_OP = 'DELETE' THEN
        UPDATE posts SET like_count = like_count - 1 WHERE id = OLD.post_id;
        RETURN OLD;
    END IF;
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
EOF

    print_success "触发器函数创建成功"
}

# 创建触发器
create_triggers() {
    print_info "创建触发器..."

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 关注关系触发器
DROP TRIGGER IF EXISTS trg_follows_insert ON follows;
DROP TRIGGER IF EXISTS trg_follows_delete ON follows;

CREATE TRIGGER trg_follows_insert
    AFTER INSERT ON follows
    FOR EACH ROW
    EXECUTE FUNCTION update_user_follow_stats();

CREATE TRIGGER trg_follows_delete
    AFTER DELETE ON follows
    FOR EACH ROW
    EXECUTE FUNCTION update_user_follow_stats();

-- 评论触发器
DROP TRIGGER IF EXISTS trg_comments_insert ON comments;
DROP TRIGGER IF EXISTS trg_comments_delete ON comments;

CREATE TRIGGER trg_comments_insert
    AFTER INSERT ON comments
    FOR EACH ROW
    EXECUTE FUNCTION update_post_stats();

CREATE TRIGGER trg_comments_delete
    AFTER DELETE ON comments
    FOR EACH ROW
    EXECUTE FUNCTION update_post_stats();

-- 点赞触发器
DROP TRIGGER IF EXISTS trg_likes_insert ON likes;
DROP TRIGGER IF EXISTS trg_likes_delete ON likes;

CREATE TRIGGER trg_likes_insert
    AFTER INSERT ON likes
    FOR EACH ROW
    EXECUTE FUNCTION update_like_stats();

CREATE TRIGGER trg_likes_delete
    AFTER DELETE ON likes
    FOR EACH ROW
    EXECUTE FUNCTION update_like_stats();
EOF

    print_success "触发器创建成功"
}

# 插入测试数据
insert_test_data() {
    print_info "插入测试数据..."

    read -p "是否插入测试数据？(y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "跳过测试数据插入"
        return
    fi

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
-- 插入测试用户
INSERT INTO users (username, email, password, display_name, bio) VALUES
    ('testuser1', 'test1@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', '测试用户1', '这是我的个人简介'),
    ('testuser2', 'test2@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', '测试用户2', '喜欢分享生活'),
    ('testuser3', 'test3@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', '测试用户3', '技术爱好者'),
    ('testuser4', 'test4@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', '测试用户4', '摄影师'),
    ('testuser5', 'test5@example.com', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', '测试用户5', '旅行达人');
EOF

    print_success "测试数据插入完成"
}

# 验证数据库结构
validate_schema() {
    print_info "验证数据库结构..."

    # 检查表是否存在
    tables=("users" "follows" "posts" "timelines" "likes" "comments")

    for table in "${tables[@]}"; do
        if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT 1 FROM information_schema.tables WHERE table_name='$table';" | grep -q 1; then
            print_success "表 $table 存在"
        else
            print_error "表 $table 不存在"
            exit 1
        fi
    done

    # 检查索引
    print_info "检查数据库索引..."
    index_count=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -tAc "SELECT COUNT(*) FROM pg_indexes WHERE schemaname = 'public';")
    print_success "数据库索引数量: $index_count"

    print_success "数据库结构验证通过"
}

# 显示数据库统计信息
show_stats() {
    print_info "数据库统计信息:"

    PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" << 'EOF'
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size,
    n_live_tup AS row_count
FROM pg_stat_user_tables
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
EOF
}

# 主函数
main() {
    print_info "开始初始化Feed流系统数据库..."
    echo "连接信息:"
    echo "  Host: $DB_HOST"
    echo "  Port: $DB_PORT"
    echo "  Database: $DB_NAME"
    echo "  User: $DB_USER"
    echo ""

    # 检查依赖
    if ! command -v psql > /dev/null 2>&1; then
        print_error "psql命令未找到，请先安装PostgreSQL客户端"
        exit 1
    fi

    # 执行初始化步骤
    check_postgres_connection
    create_database
    create_extensions
    create_users_table
    create_follows_table
    create_posts_table
    create_timelines_table
    create_likes_table
    create_comments_table
    create_trigger_functions
    create_triggers
    insert_test_data
    validate_schema
    show_stats

    echo ""
    print_success "数据库初始化完成！"
    print_info "数据库已准备就绪，可以开始使用Feed流系统"
}

# 显示帮助信息
show_help() {
    echo "Feed流系统数据库初始化脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示帮助信息"
    echo "  -d, --drop     删除现有数据库并重新创建"
    echo ""
    echo "环境变量:"
    echo "  DB_HOST        数据库主机 (默认: localhost)"
    echo "  DB_PORT        数据库端口 (默认: 5432)"
    echo "  DB_NAME        数据库名称 (默认: feedsystem)"
    echo "  DB_USER        数据库用户 (默认: feeduser)"
    echo "  DB_PASSWORD    数据库密码 (默认: feedpass)"
    echo ""
    echo "示例:"
    echo "  $0                                    # 使用默认配置"
    echo "  DB_HOST=192.168.1.100 $0             # 指定数据库主机"
    echo "  DB_NAME=myfeed $0                    # 指定数据库名称"
    echo "  $0 --drop                             # 删除并重新创建数据库"
}

# 参数处理
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -d|--drop)
        print_warning "这将删除现有数据库并重新创建！"
        read -p "确认要继续吗？(y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d postgres -c "DROP DATABASE IF EXISTS $DB_NAME;"
            print_success "数据库已删除"
        else
            print_info "操作已取消"
            exit 0
        fi
        ;;
esac

# 运行主函数
main