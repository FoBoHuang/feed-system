#!/bin/bash

# Feed流系统性能测试脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# API基础URL
BASE_URL="http://localhost:8080/api/v1"

# 测试配置
CONCURRENT_USERS=50
REQUESTS_PER_USER=20
TOTAL_REQUESTS=$((CONCURRENT_USERS * REQUESTS_PER_USER))

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

# 检查依赖
check_dependencies() {
    print_info "检查测试依赖..."

    if ! command -v curl > /dev/null 2>&1; then
        print_error "curl未安装，请先安装curl"
        exit 1
    fi

    if ! command -v jq > /dev/null 2>&1; then
        print_error "jq未安装，请先安装jq"
        exit 1
    fi

    if ! command -v bc > /dev/null 2>&1; then
        print_error "bc未安装，请先安装bc"
        exit 1
    fi

    print_success "依赖检查通过"
}

# 检查服务状态
check_service() {
    print_info "检查服务状态..."

    if ! curl -s -f "${BASE_URL}/health" > /dev/null; then
        print_error "服务未运行，请先启动服务"
        exit 1
    fi

    print_success "服务运行正常"
}

# 创建测试用户
create_test_users() {
    print_info "创建测试用户..."

    users_file="/tmp/benchmark_users.json"
    echo "[]" > "$users_file"

    for i in $(seq 1 $CONCURRENT_USERS); do
        username="bench_user_${i}_$(date +%s)"
        email="bench_${i}_$(date +%s)@example.com"
        password="password123"
        display_name="性能测试用户${i}"

        response=$(curl -s -X POST "${BASE_URL}/users/register" \
            -H "Content-Type: application/json" \
            -d "{
                \"username\": \"${username}\",
                \"email\": \"${email}\",
                \"password\": \"${password}\",
                \"display_name\": \"${display_name}\"
            }")

        if echo "$response" | grep -q '"code": 201'; then
            user_id=$(echo "$response" | jq -r '.data.user.id')

            # 用户登录
            login_response=$(curl -s -X POST "${BASE_URL}/users/login" \
                -H "Content-Type: application/json" \
                -d "{
                    \"username\": \"${username}\",
                    \"password\": \"${password}\"
                }")

            if echo "$login_response" | grep -q '"token"'; then
                token=$(echo "$login_response" | jq -r '.data.token')

                # 保存用户信息
                jq --arg id "$user_id" \
                   --arg username "$username" \
                   --arg token "$token" \
                   '. += [{"id": $id, "username": $username, "token": $token}]' \
                   "$users_file" > /tmp/temp_users.json && mv /tmp/temp_users.json "$users_file"
            fi
        fi
    done

    user_count=$(jq '. | length' "$users_file")
    print_success "创建测试用户完成: ${user_count}个用户"
}

# 创建关注关系
create_follow_relationships() {
    print_info "创建关注关系..."

    users_file="/tmp/benchmark_users.json"

    # 每个用户关注其他10个用户
    for i in $(seq 0 $((CONCURRENT_USERS - 1))); do
        user=$(jq ".[$i]" "$users_file")
        user_id=$(echo "$user" | jq -r '.id')
        token=$(echo "$user" | jq -r '.token')

        # 关注其他用户
        for j in $(seq 1 10); do
            target_index=$(( (i + j) % CONCURRENT_USERS ))
            target_user=$(jq ".[$target_index]" "$users_file")
            target_id=$(echo "$target_user" | jq -r '.id')

            if [ "$user_id" != "$target_id" ]; then
                curl -s -X POST "${BASE_URL}/users/follow" \
                    -H "Authorization: Bearer ${token}" \
                    -H "Content-Type: application/json" \
                    -d "{
                        \"user_id\": \"${user_id}\",
                        \"following_id\": \"${target_id}\"
                    }" > /dev/null
            fi
        done
    done

    print_success "关注关系创建完成"
}

# 发布测试帖子
create_test_posts() {
    print_info "发布测试帖子..."

    users_file="/tmp/benchmark_users.json"
    posts_file="/tmp/benchmark_posts.json"
    echo "[]" > "$posts_file"

    # 每个用户发布5个帖子
    for i in $(seq 0 $((CONCURRENT_USERS - 1))); do
        user=$(jq ".[$i]" "$users_file")
        token=$(echo "$user" | jq -r '.token')

        for j in $(seq 1 5); do
            content="这是性能测试帖子 ${j}，来自用户 $(echo "$user" | jq -r '.username')"

            response=$(curl -s -X POST "${BASE_URL}/posts" \
                -H "Authorization: Bearer ${token}" \
                -H "Content-Type: application/json" \
                -d "{
                    \"content\": \"${content}\",
                    \"image_urls\": []
                }")

            if echo "$response" | grep -q '"code": 201'; then
                post_id=$(echo "$response" | jq -r '.data.post.id')
                jq --arg id "$post_id" '. += [$id]' "$posts_file" > /tmp/temp_posts.json && mv /tmp/temp_posts.json "$posts_file"
            fi
        done
    done

    post_count=$(jq '. | length' "$posts_file")
    print_success "发布测试帖子完成: ${post_count}个帖子"
}

# 性能测试 - 获取Feed
benchmark_get_feed() {
    print_info "开始Feed流性能测试..."

    users_file="/tmp/benchmark_users.json"
    results_file="/tmp/benchmark_feed_results.txt"
    > "$results_file"

    start_time=$(date +%s.%N)

    # 并发测试
    for i in $(seq 0 $((CONCURRENT_USERS - 1))); do
        (
            user=$(jq ".[$i]" "$users_file")
            token=$(echo "$user" | jq -r '.token')

            for j in $(seq 1 $REQUESTS_PER_USER); do
                request_start=$(date +%s.%N)

                response=$(curl -s -w "%{http_code},%{time_total}" -X GET "${BASE_URL}/feed?limit=20" \
                    -H "Authorization: Bearer ${token}")

                request_end=$(date +%s.%N)

                http_code=$(echo "$response" | cut -d',' -f1)
                response_time=$(echo "$response" | cut -d',' -f2)

                if [ "$http_code" = "200" ]; then
                    echo "$response_time" >> "$results_file"
                else
                    echo "error" >> "$results_file"
                fi
            done
        ) &
    done

    wait

    end_time=$(date +%s.%N)
    total_time=$(echo "$end_time - $start_time" | bc)

    # 统计结果
    success_count=$(grep -v "error" "$results_file" | wc -l)
    error_count=$(grep "error" "$results_file" | wc -l)

    if [ "$success_count" -gt 0 ]; then
        avg_response_time=$(grep -v "error" "$results_file" | awk '{sum+=$1} END {print sum/NR}' | bc -l)
        min_response_time=$(grep -v "error" "$results_file" | sort -n | head -1)
        max_response_time=$(grep -v "error" "$results_file" | sort -n | tail -1)

        qps=$(echo "scale=2; $success_count / $total_time" | bc -l)

        print_success "Feed流性能测试结果:"
        echo "  总请求数: $TOTAL_REQUESTS"
        echo "  成功请求: $success_count"
        echo "  失败请求: $error_count"
        echo "  总耗时: ${total_time}s"
        echo "  QPS: $qps"
        echo "  平均响应时间: ${avg_response_time}s"
        echo "  最小响应时间: ${min_response_time}s"
        echo "  最大响应时间: ${max_response_time}s"
    else
        print_error "所有请求都失败了"
    fi
}

# 性能测试 - 创建帖子
benchmark_create_post() {
    print_info "开始创建帖子性能测试..."

    users_file="/tmp/benchmark_users.json"
    results_file="/tmp/benchmark_post_results.txt"
    > "$results_file"

    start_time=$(date +%s.%N)

    # 并发测试
    for i in $(seq 0 $((CONCURRENT_USERS - 1))); do
        (
            user=$(jq ".[$i]" "$users_file")
            token=$(echo "$user" | jq -r '.token')
            username=$(echo "$user" | jq -r '.username')

            for j in $(seq 1 $REQUESTS_PER_USER); do
                content="性能测试帖子 ${j}，来自 ${username}，时间 $(date +%s)"

                request_start=$(date +%s.%N)

                response=$(curl -s -w "%{http_code},%{time_total}" -X POST "${BASE_URL}/posts" \
                    -H "Authorization: Bearer ${token}" \
                    -H "Content-Type: application/json" \
                    -d "{
                        \"content\": \"${content}\",
                        \"image_urls\": []
                    }")

                request_end=$(date +%s.%N)

                http_code=$(echo "$response" | cut -d',' -f1)
                response_time=$(echo "$response" | cut -d',' -f2)

                if [ "$http_code" = "201" ]; then
                    echo "$response_time" >> "$results_file"
                else
                    echo "error" >> "$results_file"
                fi
            done
        ) &
    done

    wait

    end_time=$(date +%s.%N)
    total_time=$(echo "$end_time - $start_time" | bc)

    # 统计结果
    success_count=$(grep -v "error" "$results_file" | wc -l)
    error_count=$(grep "error" "$results_file" | wc -l)

    if [ "$success_count" -gt 0 ]; then
        avg_response_time=$(grep -v "error" "$results_file" | awk '{sum+=$1} END {print sum/NR}' | bc -l)
        min_response_time=$(grep -v "error" "$results_file" | sort -n | head -1)
        max_response_time=$(grep -v "error" "$results_file" | sort -n | tail -1)

        qps=$(echo "scale=2; $success_count / $total_time" | bc -l)

        print_success "创建帖子性能测试结果:"
        echo "  总请求数: $TOTAL_REQUESTS"
        echo "  成功请求: $success_count"
        echo "  失败请求: $error_count"
        echo "  总耗时: ${total_time}s"
        echo "  QPS: $qps"
        echo "  平均响应时间: ${avg_response_time}s"
        echo "  最小响应时间: ${min_response_time}s"
        echo "  最大响应时间: ${max_response_time}s"
    else
        print_error "所有请求都失败了"
    fi
}

# 性能测试 - 点赞操作
benchmark_like_post() {
    print_info "开始点赞性能测试..."

    users_file="/tmp/benchmark_users.json"
    posts_file="/tmp/benchmark_posts.json"
    results_file="/tmp/benchmark_like_results.txt"
    > "$results_file"

    # 获取帖子ID列表
    post_ids=$(jq -r '.[]' "$posts_file")
    post_array=($post_ids)
    post_count=${#post_array[@]}

    if [ "$post_count" -eq 0 ]; then
        print_warning "没有可用的测试帖子，跳过点赞测试"
        return
    fi

    start_time=$(date +%s.%N)

    # 并发测试
    for i in $(seq 0 $((CONCURRENT_USERS - 1))); do
        (
            user=$(jq ".[$i]" "$users_file")
            token=$(echo "$user" | jq -r '.token')

            for j in $(seq 1 $REQUESTS_PER_USER); do
                # 随机选择一个帖子
                random_index=$((RANDOM % post_count))
                post_id=${post_array[$random_index]}

                request_start=$(date +%s.%N)

                response=$(curl -s -w "%{http_code},%{time_total}" -X POST "${BASE_URL}/posts/${post_id}/like" \
                    -H "Authorization: Bearer ${token}")

                request_end=$(date +%s.%N)

                http_code=$(echo "$response" | cut -d',' -f1)
                response_time=$(echo "$response" | cut -d',' -f2)

                if [ "$http_code" = "200" ]; then
                    echo "$response_time" >> "$results_file"
                else
                    echo "error" >> "$results_file"
                fi
            done
        ) &
    done

    wait

    end_time=$(date +%s.%N)
    total_time=$(echo "$end_time - $start_time" | bc)

    # 统计结果
    success_count=$(grep -v "error" "$results_file" | wc -l)
    error_count=$(grep "error" "$results_file" | wc -l)

    if [ "$success_count" -gt 0 ]; then
        avg_response_time=$(grep -v "error" "$results_file" | awk '{sum+=$1} END {print sum/NR}' | bc -l)
        min_response_time=$(grep -v "error" "$results_file" | sort -n | head -1)
        max_response_time=$(grep -v "error" "$results_file" | sort -n | tail -1)

        qps=$(echo "scale=2; $success_count / $total_time" | bc -l)

        print_success "点赞性能测试结果:"
        echo "  总请求数: $TOTAL_REQUESTS"
        echo "  成功请求: $success_count"
        echo "  失败请求: $error_count"
        echo "  总耗时: ${total_time}s"
        echo "  QPS: $qps"
        echo "  平均响应时间: ${avg_response_time}s"
        echo "  最小响应时间: ${min_response_time}s"
        echo "  最大响应时间: ${max_response_time}s"
    else
        print_error "所有请求都失败了"
    fi
}

# 系统资源监控
monitor_resources() {
    print_info "监控系统资源..."

    # CPU使用率
    cpu_usage=$(top -bn1 | grep "Cpu(s)" | sed "s/.*, *\([0-9.]*\)%* id.*/\1/" | awk '{print 100 - $1}')

    # 内存使用率
    memory_info=$(free | grep Mem)
    total_memory=$(echo "$memory_info" | awk '{print $2}')
    used_memory=$(echo "$memory_info" | awk '{print $3}')
    memory_usage=$(echo "scale=2; $used_memory * 100 / $total_memory" | bc -l)

    # 磁盘使用率
    disk_usage=$(df -h / | awk 'NR==2 {print $5}' | sed 's/%//')

    print_success "系统资源监控:"
    echo "  CPU使用率: ${cpu_usage}%"
    echo "  内存使用率: ${memory_usage}%"
    echo "  磁盘使用率: ${disk_usage}%"
}

# 清理测试数据
cleanup() {
    print_info "清理测试数据..."
    rm -f /tmp/benchmark_*.json /tmp/benchmark_*.txt
    print_success "清理完成"
}

# 主测试流程
main() {
    print_info "开始Feed流系统性能测试..."
    print_info "测试配置:"
    echo "  并发用户数: $CONCURRENT_USERS"
    echo "  每用户请求数: $REQUESTS_PER_USER"
    echo "  总请求数: $TOTAL_REQUESTS"
    echo ""

    # 检查依赖
    check_dependencies

    # 检查服务
    check_service

    # 准备测试数据
    create_test_users
    create_follow_relationships
    create_test_posts

    echo ""
    print_info "开始性能测试..."
    echo "========================================"

    # 执行性能测试
    benchmark_get_feed
    echo ""
    benchmark_create_post
    echo ""
    benchmark_like_post
    echo ""

    # 监控系统资源
    monitor_resources

    echo ""
    echo "========================================"
    print_success "性能测试完成！"

    # 清理
    cleanup
}

# 显示帮助信息
show_help() {
    echo "Feed流系统性能测试脚本"
    echo ""
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  -h, --help     显示帮助信息"
    echo "  -c, --clean    清理测试数据"
    echo ""
    echo "环境变量:"
    echo "  CONCURRENT_USERS     并发用户数 (默认: 50)"
    echo "  REQUESTS_PER_USER    每用户请求数 (默认: 20)"
    echo "  BASE_URL            API基础URL (默认: http://localhost:8080/api/v1)"
    echo ""
    echo "示例:"
    echo "  $0                                    # 运行默认测试"
    echo "  CONCURRENT_USERS=100 $0              # 100并发用户测试"
    echo "  BASE_URL=http://api.example.com $0   # 指定API地址"
}

# 参数处理
case "${1:-}" in
    -h|--help)
        show_help
        exit 0
        ;;
    -c|--clean)
        cleanup
        exit 0
        ;;
esac

# 运行主函数
main