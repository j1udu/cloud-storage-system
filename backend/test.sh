#!/usr/bin/env bash
# 云盘系统后端接口测试脚本
# 用法: bash test.sh [BASE_URL]
# 示例: bash test.sh http://localhost:8080

set -euo pipefail

BASE="${1:-http://localhost:8080}"
API="$BASE/api/v1"
PASS=0
FAIL=0
SKIP=0
TOTAL=0

# ── 颜色 ──────────────────────────────────────────────
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

# ── 工具函数 ──────────────────────────────────────────
section() {
    echo -e "\n${BOLD}${CYAN}━━━ $1 ━━━${RESET}\n"
}

assert() {
    local name="$1" ok="$2"
    TOTAL=$((TOTAL + 1))
    if [ "$ok" = "true" ]; then
        echo -e "  ${GREEN}PASS${RESET} $name"
        PASS=$((PASS + 1))
    else
        echo -e "  ${RED}FAIL${RESET} $name"
        FAIL=$((FAIL + 1))
    fi
}

skip() {
    local name="$1" reason="$2"
    TOTAL=$((TOTAL + 1))
    echo -e "  ${YELLOW}SKIP${RESET} $name ($reason)"
    SKIP=$((SKIP + 1))
}

# api METHOD PATH [BODY] — 发请求并打印响应
api() {
    local method="$1" path="$2" body="${3:-}"
    local args=(-s -X "$method" "$API$path")
    if [ -n "$TOKEN" ]; then
        args+=(-H "Authorization: Bearer $TOKEN")
    fi
    if [ -n "$body" ]; then
        args+=(-H "Content-Type: application/json" -d "$body")
    fi
    curl "${args[@]}"
}

# api_raw METHOD PATH [EXTRA_CURL_ARGS...] — 用于文件上传等非 JSON 场景
api_raw() {
    curl -s -H "Authorization: Bearer $TOKEN" "$@"
}

check_code() {
    local response="$1" expected="$2"
    local actual
    actual=$(echo "$response" | jq -r '.code')
    [ "$actual" = "$expected" ]
}

check_msg() {
    local response="$1" expected="$2"
    echo "$response" | jq -r '.msg' | grep -q "$expected"
}

# ── 依赖检查 ──────────────────────────────────────────
for cmd in curl jq mktemp; do
    if ! command -v "$cmd" &>/dev/null; then
        echo -e "${RED}错误: 需要安装 $cmd${RESET}"
        exit 1
    fi
done

# ── 临时文件 & 清理 ───────────────────────────────────
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

echo -e "${BOLD}云盘后端接口测试${RESET}"
echo -e "目标: $BASE\n"

# ══════════════════════════════════════════════════════
section "1. 健康检查"
# ══════════════════════════════════════════════════════

RES=$(curl -s "$BASE/ping")
assert "Ping 返回 code=0" "$(check_code "$RES" 0)"

# ══════════════════════════════════════════════════════
section "2. 用户注册"
# ══════════════════════════════════════════════════════

USERNAME="test_$(date +%s)"
PASSWORD="test123456"
TOKEN=""

# 2.1 正常注册
RES=$(api POST /auth/register "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
assert "正常注册" "$(check_code "$RES" 0)"
assert "注册返回用户名" "$(echo "$RES" | jq -r '.data.username') = $USERNAME"

# 2.2 重复用户名
RES=$(api POST /auth/register "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
assert "重复用户名注册失败" "$(check_code "$RES" "1")"

# 2.3 缺少参数
RES=$(api POST /auth/register '{"username":""}')
assert "空用户名注册失败" "$(check_code "$RES" "1")"

RES=$(api POST /auth/register '{"username":"a","password":"123"}')
assert "短密码注册失败" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "3. 用户登录"
# ══════════════════════════════════════════════════════

# 3.1 正常登录
RES=$(api POST /auth/login "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")
assert "正常登录" "$(check_code "$RES" 0)"
TOKEN=$(echo "$RES" | jq -r '.data.token')
EXPIRES=$(echo "$RES" | jq -r '.data.expires_at')
assert "返回 token" "[ $TOKEN != 'null' ] && [ -n \"$TOKEN\" ]"
assert "返回过期时间" "[ $EXPIRES != 'null' ]"

# 3.2 错误密码
RES=$(api POST /auth/login "{\"username\":\"$USERNAME\",\"password\":\"wrong\"}")
assert "错误密码登录失败" "$(check_code "$RES" "1")"

# 3.3 不存在的用户
RES=$(api POST /auth/login '{"username":"nonexistent_user_12345","password":"whatever"}')
assert "不存在用户登录失败" "$(check_code "$RES" "1")"

# 3.4 保存第二个用户的 token 用于后续权限测试
USER2="test2_$(date +%s)"
api POST /auth/register "{\"username\":\"$USER2\",\"password\":\"$PASSWORD\"}" >/dev/null
RES2=$(api POST /auth/login "{\"username\":\"$USER2\",\"password\":\"$PASSWORD\"}")
TOKEN2=$(echo "$RES2" | jq -r '.data.token')

# ══════════════════════════════════════════════════════
section "4. 获取用户信息"
# ══════════════════════════════════════════════════════

# 4.1 正常获取
RES=$(api GET /auth/profile)
assert "获取用户信息" "$(check_code "$RES" 0)"
assert "返回正确用户名" "$(echo "$RES" | jq -r '.data.username') = $USERNAME"

# 4.2 无 token
RES=$(curl -s "$API/auth/profile")
assert "无 token 访问 profile 失败" "$(check_code "$RES "1")"

# 4.3 错误 token
RES=$(curl -s -H "Authorization: Bearer invalid.token.here" "$API/auth/profile")
assert "错误 token 访问 profile 失败" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "5. 文件上传"
# ══════════════════════════════════════════════════════

# 5.1 创建测试文件
echo "Hello Cloud Storage - $(date)" > "$TMPDIR/test1.txt"
echo "Second file content" > "$TMPDIR/test2.txt"

# 5.2 上传到根目录
RES=$(api_raw -X POST "$API/files/upload" \
    -F "file=@$TMPDIR/test1.txt" \
    -F "parent_id=0")
assert "上传文件到根目录" "$(check_code "$RES" 0)"
FILE1_ID=$(echo "$RES" | jq -r '.data.id')
assert "返回文件 ID" "[ $FILE1_ID != 'null' ] && [ $FILE1_ID -gt 0 ]"

# 5.3 上传第二个文件
RES=$(api_raw -X POST "$API/files/upload" \
    -F "file=@$TMPDIR/test2.txt" \
    -F "parent_id=0")
assert "上传第二个文件" "$(check_code "$RES" 0)"
FILE2_ID=$(echo "$RES" | jq -r '.data.id')

# 5.4 不带文件上传
RES=$(api_raw -X POST "$API/files/upload" -F "parent_id=0")
assert "不传文件上传失败" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "6. 文件列表"
# ══════════════════════════════════════════════════════

# 6.1 列出根目录
RES=$(api GET "/files?folder_id=0&page=1&page_size=20")
assert "列出根目录文件" "$(check_code "$RES" 0)"
assert "文件数量 >= 2" "$(echo "$RES" | jq '.data.total') -ge 2"

# 6.2 分页
RES=$(api GET "/files?folder_id=0&page=1&page_size=1")
assert "分页查询 page_size=1" "$(check_code "$RES" 0)"
assert "分页后 items 只有 1 条" "$(echo "$RES" | jq '.data.items | length') = 1"

# ══════════════════════════════════════════════════════
section "7. 文件下载"
# ══════════════════════════════════════════════════════

# 7.1 正常下载
RES=$(api GET "/files/$FILE1_ID/download")
assert "获取下载链接" "$(check_code "$RES" 0)"
assert "返回 url 字段" "$(echo "$RES" | jq -r '.data.url') != 'null'"

# 7.2 不存在的文件
RES=$(api GET "/files/999999/download")
assert "下载不存在的文件失败" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "8. 文件重命名"
# ══════════════════════════════════════════════════════

# 8.1 正常重命名
RES=$(api PUT "/files/$FILE1_ID/rename" '{"name":"renamed_file.txt"}')
assert "重命名文件" "$(check_code "$RES" 0)"

# 8.2 验证新名字
RES=$(api GET "/files?folder_id=0&page=1&page_size=50")
assert "列表中出现新名字" "$(echo "$RES" | jq -r '.data.items[].name') | grep -q 'renamed_file.txt'"

# 8.3 空名称
RES=$(api PUT "/files/$FILE1_ID/rename" '{"name":""}')
assert "空名称重命名失败" "$(check_code "$RES" "1")"

# 8.4 不存在的文件
RES=$(api PUT "/files/999999/rename" '{"name":"test"}')
assert "重命名不存在的文件失败" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "9. 文件夹操作"
# ══════════════════════════════════════════════════════

# 9.1 创建文件夹
RES=$(api POST /folders '{"parent_id":0,"name":"测试文件夹"}')
assert "在根目录创建文件夹" "$(check_code "$RES" 0)"
FOLDER_ID=$(echo "$RES" | jq -r '.data.id')
assert "返回文件夹 ID" "[ $FOLDER_ID != 'null' ] && [ $FOLDER_ID -gt 0 ]"

# 9.2 同名文件夹
RES=$(api POST /folders '{"parent_id":0,"name":"测试文件夹"}')
assert "同名文件夹创建失败" "$(check_code "$RES" "1")"

# 9.3 空名称
RES=$(api POST /folders '{"parent_id":0,"name":""}')
assert "空名称创建文件夹失败" "$(check_code "$RES" "1")"

# 9.4 不存在的父目录
RES=$(api POST /folders '{"parent_id":999999,"name":"orphan"}')
assert "父目录不存在时创建失败" "$(check_code "$RES" "1")"

# 9.5 在文件夹内创建子文件夹
RES=$(api POST /folders "{\"parent_id\":$FOLDER_ID,\"name\":\"子文件夹\"}")
assert "创建子文件夹" "$(check_code "$RES" 0)"
SUBFOLDER_ID=$(echo "$RES" | jq -r '.data.id')

# ══════════════════════════════════════════════════════
section "10. 移动文件"
# ══════════════════════════════════════════════════════

# 10.1 把 file2 移动到文件夹内
RES=$(api PUT "/files/$FILE2_ID/move" "{\"target_id\":$FOLDER_ID}")
assert "移动文件到文件夹" "$(check_code "$RES" 0)"

# 10.2 验证移动结果：根目录少一个文件，文件夹内多一个
RES=$(api GET "/files?folder_id=0&page=1&page_size=50")
ROOT_COUNT=$(echo "$RES" | jq '.data.total')
RES=$(api GET "/files?folder_id=$FOLDER_ID&page=1&page_size=50")
FOLDER_COUNT=$(echo "$RES" | jq '.data.total')
assert "文件夹内有文件" "[ $FOLDER_COUNT -ge 1 ]"

# 10.3 移动到自身
RES=$(api PUT "/files/$FILE1_ID/move" "{\"target_id\":$FILE1_ID}")
assert "移动到自身失败" "$(check_code "$RES" "1")"

# 10.4 把文件夹移到自己的子文件夹（循环引用）
RES=$(api PUT "/files/$FOLDER_ID/move" "{\"target_id\":$SUBFOLDER_ID}")
assert "循环引用移动失败" "$(check_code "$RES" "1")"

# 10.5 移动到不存在的文件夹
RES=$(api PUT "/files/$FILE1_ID/move" '{"target_id":999999}')
assert "移动到不存在的目标失败" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "11. 面包屑路径"
# ══════════════════════════════════════════════════════

# 11.1 根目录路径
RES=$(api GET "/folders/path?folder_id=0")
assert "根目录路径" "$(check_code "$RES" 0)"
assert "根目录路径只有一项" "$(echo "$RES" | jq '.data | length') = 1"
assert "根目录名称正确" "$(echo "$RES" | jq -r '.data[0].name') = '根目录'"

# 11.2 子文件夹路径
RES=$(api GET "/folders/path?folder_id=$SUBFOLDER_ID")
assert "子文件夹路径" "$(check_code "$RES" 0)"
assert "路径层级 >= 3（根 → 父 → 子）" "$(echo "$RES" | jq '.data | length') -ge 3"

# 11.3 无效 folder_id
RES=$(api GET "/folders/path?folder_id=abc")
assert "无效 folder_id 返回错误" "$(check_code "$RES" "1")"

# ══════════════════════════════════════════════════════
section "12. 跨用户权限"
# ══════════════════════════════════════════════════════

# 用 user2 的 token 操作 user1 的文件
SAVED_TOKEN="$TOKEN"
TOKEN="$TOKEN2"

# 12.1 下载别人的文件
RES=$(api GET "/files/$FILE1_ID/download")
assert "无权下载他人文件" "$(check_code "$RES" "1")"

# 12.2 删除别人的文件
RES=$(api DELETE "/files/$FILE1_ID")
assert "无权删除他人文件" "$(check_code "$RES" "1")"

# 12.3 重命名别人的文件
RES=$(api PUT "/files/$FILE1_ID/rename" '{"name":"hacked"}')
assert "无权重命名他人文件" "$(check_code "$RES" "1")"

# 12.4 移动别人的文件
RES=$(api PUT "/files/$FILE1_ID/move" '{"target_id":0}')
assert "无权移动他人文件" "$(check_code "$RES" "1")"

# 恢复原 token
TOKEN="$SAVED_TOKEN"

# ══════════════════════════════════════════════════════
section "13. 文件删除"
# ══════════════════════════════════════════════════════

# 13.1 删除文件
RES=$(api DELETE "/files/$FILE1_ID")
assert "删除文件" "$(check_code "$RES" 0)"

# 13.2 重复删除（软删除后再删）
RES=$(api DELETE "/files/$FILE1_ID")
assert "重复删除应报错或成功" "$(check_code "$RES" 0 || check_code "$RES" 1)"

# 13.3 删除不存在的文件
RES=$(api DELETE "/files/999999")
assert "删除不存在的文件失败" "$(check_code "$RES" "1")"

# 13.4 删除文件夹
RES=$(api DELETE "/files/$SUBFOLDER_ID")
assert "删除子文件夹" "$(check_code "$RES" 0)"

# ══════════════════════════════════════════════════════
section "14. 用户登出"
# ══════════════════════════════════════════════════════

RES=$(api POST /auth/logout)
assert "登出成功" "$(check_code "$RES" 0)"

# ══════════════════════════════════════════════════════
# 测试结果汇总
# ══════════════════════════════════════════════════════
echo -e "\n${BOLD}═══════════════════════════════════════${RESET}"
echo -e "${BOLD}测试结果: $TOTAL 项${RESET}"
echo -e "  ${GREEN}通过: $PASS${RESET}"
echo -e "  ${RED}失败: $FAIL${RESET}"
echo -e "  ${YELLOW}跳过: $SKIP${RESET}"
echo -e "${BOLD}═══════════════════════════════════════${RESET}"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
