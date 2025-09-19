#!/bin/bash

# API测试脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# API基础URL
BASE_URL="http://localhost:8080/api/v1"

# 测试数据
TEST_USERNAME="testuser_$(date +%s)"
TEST_EMAIL="test$(date +%s)@example.com"
TEST_PASSWORD="password123"
TEST_DISPLAY_NAME="测试用户"

# 输出函数
print_info() {
    echo -e "${YELLOW}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查服务是否运行
check_service() {
    print_info "检查服务状态..."

    if ! curl -s -f "${BASE_URL}/health" > /dev/null; then
        print_error "服务未运行，请先启动服务"
        exit 1
    fi

    print_success "服务运行正常"
}

# 用户注册测试
test_user_registration() {
    print_info "测试用户注册..."

    response=$(curl -s -X POST "${BASE_URL}/users/register" \
        -H "Content-Type: application/json" \
        -d "{
            \"username\": \"${TEST_USERNAME}\",
            \"email\": \"${TEST_EMAIL}\",
            \"password\": \"${TEST_PASSWORD}\",
            \"display_name\": \"${TEST_DISPLAY_NAME}\"
        }")

    if echo "$response" | grep -q '"code": 201'; then
        print_success "用户注册成功"
        echo "$response" | jq '.data.user' > /tmp/test_user.json
    else
        print_error "用户注册失败: $response"
        exit 1
    fi
}

# 用户登录测试
test_user_login() {
    print_info "测试用户登录..."

    response=$(curl -s -X POST "${BASE_URL}/users/login" \
        -H "Content-Type: application/json" \
        -d "{
            \"username\": \"${TEST_USERNAME}\",
            \"password\": \"${TEST_PASSWORD}\"
        }")

    if echo "$response" | grep -q '"token"'; then
        TOKEN=$(echo "$response" | jq -r '.data.token')
        print_success "用户登录成功"
        echo "$TOKEN" > /tmp/test_token.txt
    else
        print_error "用户登录失败: $response"
        exit 1
    fi
}

# 获取用户信息测试
test_get_user_profile() {
    print_info "测试获取用户信息..."

    USER_ID=$(cat /tmp/test_user.json | jq -r '.id')

    response=$(curl -s -X GET "${BASE_URL}/users/${USER_ID}" \
        -H "Authorization: Bearer ${TOKEN}")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "获取用户信息成功"
    else
        print_error "获取用户信息失败: $response"
        exit 1
    fi
}

# 更新用户信息测试
test_update_user_profile() {
    print_info "测试更新用户信息..."

    response=$(curl -s -X PUT "${BASE_URL}/users/profile" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"display_name\": \"更新后的显示名称\",
            \"bio\": \"这是更新后的个人简介\"
        }")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "更新用户信息成功"
    else
        print_error "更新用户信息失败: $response"
        exit 1
    fi
}

# 创建帖子测试
test_create_post() {
    print_info "测试创建帖子..."

    response=$(curl -s -X POST "${BASE_URL}/posts" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"content\": \"这是一条测试动态！\",
            \"image_urls\": [\"https://example.com/test.jpg\"]
        }")

    if echo "$response" | grep -q '"code": 201'; then
        print_success "创建帖子成功"
        echo "$response" | jq '.data.post' > /tmp/test_post.json
        POST_ID=$(echo "$response" | jq -r '.data.post.id')
        echo "$POST_ID" > /tmp/test_post_id.txt
    else
        print_error "创建帖子失败: $response"
        exit 1
    fi
}

# 获取Feed测试
test_get_feed() {
    print_info "测试获取Feed流..."

    response=$(curl -s -X GET "${BASE_URL}/feed?limit=10" \
        -H "Authorization: Bearer ${TOKEN}")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "获取Feed流成功"
    else
        print_error "获取Feed流失败: $response"
        exit 1
    fi
}

# 获取帖子详情测试
test_get_post() {
    print_info "测试获取帖子详情..."

    POST_ID=$(cat /tmp/test_post_id.txt)

    response=$(curl -s -X GET "${BASE_URL}/posts/${POST_ID}" \
        -H "Authorization: Bearer ${TOKEN}")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "获取帖子详情成功"
    else
        print_error "获取帖子详情失败: $response"
        exit 1
    fi
}

# 点赞测试
test_like_post() {
    print_info "测试点赞功能..."

    POST_ID=$(cat /tmp/test_post_id.txt)

    response=$(curl -s -X POST "${BASE_URL}/posts/${POST_ID}/like" \
        -H "Authorization: Bearer ${TOKEN}")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "点赞成功"
    else
        print_error "点赞失败: $response"
        exit 1
    fi
}

# 创建评论测试
test_create_comment() {
    print_info "测试创建评论..."

    POST_ID=$(cat /tmp/test_post_id.txt)

    response=$(curl -s -X POST "${BASE_URL}/posts/${POST_ID}/comments" \
        -H "Authorization: Bearer ${TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{
            \"content\": \"这是一条测试评论\"
        }")

    if echo "$response" | grep -q '"code": 201'; then
        print_success "创建评论成功"
        COMMENT_ID=$(echo "$response" | jq -r '.data.comment.id')
        echo "$COMMENT_ID" > /tmp/test_comment_id.txt
    else
        print_error "创建评论失败: $response"
        exit 1
    fi
}

# 搜索测试
test_search() {
    print_info "测试搜索功能..."

    # 搜索用户
    response=$(curl -s -X GET "${BASE_URL}/users/search?q=test&limit=5" \
        -H "Authorization: Bearer ${TOKEN}")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "搜索用户成功"
    else
        print_error "搜索用户失败: $response"
        exit 1
    fi

    # 搜索帖子
    response=$(curl -s -X GET "${BASE_URL}/posts/search?q=测试&limit=5" \
        -H "Authorization: Bearer ${TOKEN}")

    if echo "$response" | grep -q '"code": 200'; then
        print_success "搜索帖子成功"
    else
        print_error "搜索帖子失败: $response"
        exit 1
    fi
}

# 清理测试数据
cleanup() {
    print_info "清理测试数据..."
    rm -f /tmp/test_*.json /tmp/test_*.txt /tmp/test_*.log
    print_success "清理完成"
}

# 主测试流程
main() {
    print_info "开始API测试..."

    # 检查服务
    check_service

    # 用户相关测试
    test_user_registration
    test_user_login
    test_get_user_profile
    test_update_user_profile

    # Feed相关测试
    test_create_post
    test_get_feed
    test_get_post

    # 互动相关测试
    test_like_post
    test_create_comment

    # 搜索测试
    test_search

    print_success "所有API测试通过！"

    # 清理
    cleanup
}

# 检查依赖
if ! command -v curl > /dev/null 2>&1; then
    print_error "curl未安装，请先安装curl"
    exit 1
fi

if ! command -v jq > /dev/null 2>&1; then
    print_error "jq未安装，请先安装jq"
    exit 1
fi

# 运行主函数
main

echo ""
print_success "API测试完成！"
print_info "测试用户: ${TEST_USERNAME}"
print_info "测试邮箱: ${TEST_EMAIL}"
print_info "Token已保存到: /tmp/test_token.txt" (可用于手动测试其他接口)