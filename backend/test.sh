#!/usr/bin/env bash
# Backend API smoke/integration test script.
# Usage:
#   bash test.sh [BASE_URL]
# Example:
#   bash test.sh http://localhost:8080
# Share tests:
#   powershell -ExecutionPolicy Bypass -File test_share.ps1 [BASE_URL]
# Recycle tests:
#   powershell -ExecutionPolicy Bypass -File test_recycle.ps1 [BASE_URL]
# Quota tests:
#   start backend with CLOUD_QUOTA_DEFAULT_BYTES=4096
#   powershell -ExecutionPolicy Bypass -File test_quota.ps1 [BASE_URL] 4096

set -u

BASE="${1:-http://localhost:8080}"
API="$BASE/api/v1"
TOKEN=""
TOKEN2=""
PASS=0
FAIL=0
SKIP=0
TOTAL=0

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

section() {
  printf "\n%b== %s ==%b\n" "$BOLD$CYAN" "$1" "$RESET"
}

record() {
  local status="$1"
  local name="$2"
  TOTAL=$((TOTAL + 1))

  case "$status" in
    pass)
      PASS=$((PASS + 1))
      printf "  %bPASS%b %s\n" "$GREEN" "$RESET" "$name"
      ;;
    fail)
      FAIL=$((FAIL + 1))
      printf "  %bFAIL%b %s\n" "$RED" "$RESET" "$name"
      ;;
    skip)
      SKIP=$((SKIP + 1))
      printf "  %bSKIP%b %s\n" "$YELLOW" "$RESET" "$name"
      ;;
  esac
}

assert_true() {
  local name="$1"
  shift
  if "$@"; then
    record pass "$name"
  else
    record fail "$name"
  fi
}

assert_code() {
  local name="$1"
  local response="$2"
  local expected="$3"
  local actual

  actual="$(printf "%s" "$response" | jq -r '.code // empty' 2>/dev/null)"
  if [ "$actual" = "$expected" ]; then
    record pass "$name"
  else
    record fail "$name (expected code=$expected, got code=${actual:-<missing>}, response=$response)"
  fi
}

json_field_equals() {
  local response="$1"
  local field="$2"
  local expected="$3"
  local actual

  actual="$(printf "%s" "$response" | jq -r "$field // empty" 2>/dev/null)"
  [ "$actual" = "$expected" ]
}

json_field_not_empty() {
  local response="$1"
  local field="$2"
  local actual

  actual="$(printf "%s" "$response" | jq -r "$field // empty" 2>/dev/null)"
  [ -n "$actual" ] && [ "$actual" != "null" ]
}

json_number_ge() {
  local response="$1"
  local field="$2"
  local minimum="$3"
  local actual

  actual="$(printf "%s" "$response" | jq -r "$field // 0" 2>/dev/null)"
  [ "$actual" -ge "$minimum" ] 2>/dev/null
}

response_code_is() {
  local response="$1"
  local expected="$2"
  local actual

  actual="$(printf "%s" "$response" | jq -r '.code // empty' 2>/dev/null)"
  [ "$actual" = "$expected" ]
}

json_value() {
  local response="$1"
  local field="$2"

  printf "%s" "$response" | jq -r "$field // empty" 2>/dev/null
}

json_items_has_id() {
  local response="$1"
  local id="$2"

  printf "%s" "$response" | jq -e --argjson id "$id" '.data.items[]? | select(.id == $id)' >/dev/null 2>&1
}

json_items_missing_id() {
  local response="$1"
  local id="$2"

  ! json_items_has_id "$response" "$id"
}

api() {
  local method="$1"
  local path="$2"
  local body="${3:-}"
  local args=(-sS -X "$method" "$API$path")

  if [ -n "$TOKEN" ]; then
    args+=(-H "Authorization: Bearer $TOKEN")
  fi
  if [ -n "$body" ]; then
    args+=(-H "Content-Type: application/json" -d "$body")
  fi

  curl "${args[@]}"
}

api_no_auth() {
  local method="$1"
  local path="$2"
  curl -sS -X "$method" "$API$path"
}

api_upload() {
  local file_path="$1"
  local parent_id="${2:-0}"

  curl -sS -X POST "$API/files/upload" \
    -H "Authorization: Bearer $TOKEN" \
    -F "file=@$file_path" \
    -F "parent_id=$parent_id"
}

need_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf "%bERROR%b missing dependency: %s\n" "$RED" "$RESET" "$1"
    exit 1
  fi
}

for cmd in curl jq mktemp; do
  need_command "$cmd"
done

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT

printf "%bBackend API test%b\n" "$BOLD" "$RESET"
printf "Target: %s\n" "$BASE"

section "1. Health"
RES="$(curl -sS "$BASE/ping" 2>/dev/null || true)"
assert_code "ping returns code 0" "$RES" "0"

section "2. Auth"
TS="$(date +%s)"
USERNAME="test_${TS}_$RANDOM"
PASSWORD="test123456"
USER2="test2_${TS}_$RANDOM"

RES="$(api POST /auth/register "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")"
assert_code "register user" "$RES" "0"
assert_true "register returns username" json_field_equals "$RES" ".data.username" "$USERNAME"

RES="$(api POST /auth/register "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")"
assert_code "duplicate register fails" "$RES" "10001"

RES="$(api POST /auth/register '{"username":"","password":"test123456"}')"
assert_code "empty username fails" "$RES" "10005"

RES="$(api POST /auth/register '{"username":"abc","password":"123"}')"
assert_code "short password fails" "$RES" "10005"

RES="$(api POST /auth/login "{\"username\":\"$USERNAME\",\"password\":\"$PASSWORD\"}")"
assert_code "login user" "$RES" "0"
assert_true "login returns token" json_field_not_empty "$RES" ".data.token"
assert_true "login returns expires_at" json_field_not_empty "$RES" ".data.expires_at"
TOKEN="$(printf "%s" "$RES" | jq -r '.data.token')"

RES="$(api POST /auth/login "{\"username\":\"$USERNAME\",\"password\":\"wrong123\"}")"
assert_code "wrong password fails" "$RES" "10003"

RES="$(api POST /auth/register "{\"username\":\"$USER2\",\"password\":\"$PASSWORD\"}")"
assert_code "register second user" "$RES" "0"
RES="$(api POST /auth/login "{\"username\":\"$USER2\",\"password\":\"$PASSWORD\"}")"
assert_code "login second user" "$RES" "0"
TOKEN2="$(printf "%s" "$RES" | jq -r '.data.token')"

section "3. Profile"
RES="$(api GET /auth/profile)"
assert_code "profile with token" "$RES" "0"
assert_true "profile returns username" json_field_equals "$RES" ".data.username" "$USERNAME"

RES="$(api_no_auth GET /auth/profile)"
assert_code "profile without token fails" "$RES" "10004"

section "4. Files"
printf "Hello Cloud Storage %s\n" "$(date)" > "$TMPDIR/test1.txt"
printf "Second file content\n" > "$TMPDIR/test2.txt"

RES="$(api_upload "$TMPDIR/test1.txt" 0)"
assert_code "upload first file" "$RES" "0"
assert_true "upload first file returns id" json_field_not_empty "$RES" ".data.id"
FILE1_ID="$(printf "%s" "$RES" | jq -r '.data.id')"

RES="$(api_upload "$TMPDIR/test2.txt" 0)"
assert_code "upload second file" "$RES" "0"
assert_true "upload second file returns id" json_field_not_empty "$RES" ".data.id"
FILE2_ID="$(printf "%s" "$RES" | jq -r '.data.id')"

RES="$(curl -sS -X POST "$API/files/upload" -H "Authorization: Bearer $TOKEN" -F "parent_id=0")"
assert_code "upload without file fails" "$RES" "10005"

RES="$(api GET '/files?folder_id=0&page=1&page_size=20')"
assert_code "list root files" "$RES" "0"
assert_true "root list has at least two items" json_number_ge "$RES" ".data.total" 2

RES="$(api GET '/files?folder_id=0&page=1&page_size=1')"
assert_code "list pagination" "$RES" "0"
assert_true "page size one returns one item" json_field_equals "$RES" ".data.items | length" "1"

RES="$(api GET "/files/$FILE1_ID/download")"
assert_code "get download url" "$RES" "0"
assert_true "download response has url" json_field_not_empty "$RES" ".data.url"

RES="$(api GET '/files/999999/download')"
assert_code "download missing file fails" "$RES" "10005"

RES="$(api PUT "/files/$FILE1_ID/rename" '{"name":"renamed_file.txt"}')"
assert_code "rename file" "$RES" "0"

RES="$(api GET '/files?folder_id=0&page=1&page_size=50')"
if printf "%s" "$RES" | jq -e '.data.items[].name == "renamed_file.txt"' >/dev/null 2>&1; then
  record pass "renamed file appears in list"
else
  record fail "renamed file appears in list"
fi

RES="$(api PUT "/files/$FILE1_ID/rename" '{"name":""}')"
assert_code "empty rename fails" "$RES" "10005"

section "5. Folders and move"
FOLDER_NAME="test_folder_${TS}_$RANDOM"
SUBFOLDER_NAME="sub_folder_${TS}_$RANDOM"

RES="$(api POST /folders "{\"parent_id\":0,\"name\":\"$FOLDER_NAME\"}")"
assert_code "create folder" "$RES" "0"
assert_true "create folder returns id" json_field_not_empty "$RES" ".data.id"
FOLDER_ID="$(json_value "$RES" ".data.id")"

if [ -n "$FOLDER_ID" ]; then
  RES="$(api POST /folders "{\"parent_id\":0,\"name\":\"$FOLDER_NAME\"}")"
  assert_code "duplicate folder fails" "$RES" "10005"
else
  record skip "duplicate folder fails"
fi

RES="$(api POST /folders '{"parent_id":0,"name":""}')"
assert_code "empty folder name fails" "$RES" "10005"

RES="$(api POST /folders '{"parent_id":999999,"name":"orphan"}')"
assert_code "missing parent folder fails" "$RES" "10005"

if [ -n "$FOLDER_ID" ]; then
  RES="$(api POST /folders "{\"parent_id\":$FOLDER_ID,\"name\":\"$SUBFOLDER_NAME\"}")"
  assert_code "create subfolder" "$RES" "0"
  assert_true "create subfolder returns id" json_field_not_empty "$RES" ".data.id"
  SUBFOLDER_ID="$(json_value "$RES" ".data.id")"
else
  SUBFOLDER_ID=""
  record skip "create subfolder"
  record skip "create subfolder returns id"
fi

if [ -n "$FOLDER_ID" ]; then
  RES="$(api PUT "/files/$FILE2_ID/move" "{\"target_id\":$FOLDER_ID}")"
  assert_code "move file into folder" "$RES" "0"

  RES="$(api GET "/files?folder_id=$FOLDER_ID&page=1&page_size=50")"
  assert_code "list folder after move" "$RES" "0"
  assert_true "folder has moved file" json_number_ge "$RES" ".data.total" 1
else
  record skip "move file into folder"
  record skip "list folder after move"
  record skip "folder has moved file"
fi

RES="$(api PUT "/files/$FILE1_ID/move" "{\"target_id\":$FILE1_ID}")"
assert_code "move file to itself fails" "$RES" "10005"

if [ -n "$FOLDER_ID" ] && [ -n "$SUBFOLDER_ID" ]; then
  RES="$(api PUT "/files/$FOLDER_ID/move" "{\"target_id\":$SUBFOLDER_ID}")"
  assert_code "move folder into child fails" "$RES" "10005"
else
  record skip "move folder into child fails"
fi

RES="$(api PUT "/files/$FILE1_ID/move" '{"target_id":999999}')"
assert_code "move to missing folder fails" "$RES" "10005"

section "6. Folder path"
RES="$(api GET '/folders/path?folder_id=0')"
assert_code "root path" "$RES" "0"
assert_true "root path has one item" json_field_equals "$RES" ".data | length" "1"

if [ -n "$SUBFOLDER_ID" ]; then
  RES="$(api GET "/folders/path?folder_id=$SUBFOLDER_ID")"
  assert_code "subfolder path" "$RES" "0"
  assert_true "subfolder path has at least three items" json_number_ge "$RES" ".data | length" 3
else
  record skip "subfolder path"
  record skip "subfolder path has at least three items"
fi

RES="$(api GET '/folders/path?folder_id=abc')"
assert_code "invalid folder path id fails" "$RES" "10005"

section "7. Cross-user permissions"
SAVED_TOKEN="$TOKEN"
TOKEN="$TOKEN2"

RES="$(api GET "/files/$FILE1_ID/download")"
assert_code "other user cannot download file" "$RES" "10005"

RES="$(api DELETE "/files/$FILE1_ID")"
assert_code "other user cannot delete file" "$RES" "10005"

RES="$(api PUT "/files/$FILE1_ID/rename" '{"name":"hacked.txt"}')"
assert_code "other user cannot rename file" "$RES" "10005"

RES="$(api PUT "/files/$FILE1_ID/move" '{"target_id":0}')"
assert_code "other user cannot move file" "$RES" "10005"

TOKEN="$SAVED_TOKEN"

section "8. Delete and logout"
RES="$(api DELETE "/files/$FILE1_ID")"
assert_code "delete file" "$RES" "0"

RES="$(api GET '/files?folder_id=0&page=1&page_size=50')"
assert_code "list root after delete" "$RES" "0"
assert_true "deleted file hidden from normal list" json_items_missing_id "$RES" "$FILE1_ID"

RES="$(api GET '/recycle?page=1&page_size=50')"
assert_code "list recycle after delete" "$RES" "0"
assert_true "deleted file appears in recycle" json_items_has_id "$RES" "$FILE1_ID"

RES="$(api PUT "/recycle/$FILE1_ID/restore")"
assert_code "restore file from recycle" "$RES" "0"

RES="$(api GET '/files?folder_id=0&page=1&page_size=50')"
assert_code "list root after restore" "$RES" "0"
assert_true "restored file appears in normal list" json_items_has_id "$RES" "$FILE1_ID"

RES="$(api DELETE "/files/$FILE1_ID")"
assert_code "delete file before permanent delete" "$RES" "0"

RES="$(api DELETE "/recycle/$FILE1_ID")"
assert_code "permanent delete file" "$RES" "0"

RES="$(api GET '/recycle?page=1&page_size=50')"
assert_code "list recycle after permanent delete" "$RES" "0"
assert_true "permanently deleted file hidden from recycle" json_items_missing_id "$RES" "$FILE1_ID"

RES="$(api PUT "/recycle/$FILE1_ID/restore")"
assert_code "restore permanently deleted file fails" "$RES" "10005"

RES="$(api DELETE '/files/999999')"
assert_code "delete missing file fails" "$RES" "10005"

if [ -n "$FOLDER_ID" ] && [ -n "$SUBFOLDER_ID" ]; then
  RES="$(api DELETE "/files/$FOLDER_ID")"
  assert_code "delete folder recursively" "$RES" "0"

  RES="$(api GET "/files?folder_id=$FOLDER_ID&page=1&page_size=50")"
  assert_code "list deleted folder normal children" "$RES" "0"
  assert_true "deleted folder child file hidden from normal list" json_items_missing_id "$RES" "$FILE2_ID"
  assert_true "deleted folder child folder hidden from normal list" json_items_missing_id "$RES" "$SUBFOLDER_ID"

  RES="$(api GET '/recycle?page=1&page_size=100')"
  assert_code "list recycle after folder delete" "$RES" "0"
  assert_true "deleted folder appears in recycle" json_items_has_id "$RES" "$FOLDER_ID"
  assert_true "deleted folder child file appears in recycle" json_items_has_id "$RES" "$FILE2_ID"
  assert_true "deleted folder child folder appears in recycle" json_items_has_id "$RES" "$SUBFOLDER_ID"
else
  record skip "delete folder recursively"
  record skip "list deleted folder normal children"
  record skip "deleted folder child file hidden from normal list"
  record skip "deleted folder child folder hidden from normal list"
  record skip "list recycle after folder delete"
  record skip "deleted folder appears in recycle"
  record skip "deleted folder child file appears in recycle"
  record skip "deleted folder child folder appears in recycle"
fi

RES="$(api POST /auth/logout)"
assert_code "logout" "$RES" "0"

printf "\n%bSummary%b\n" "$BOLD" "$RESET"
printf "  Total: %s\n" "$TOTAL"
printf "  %bPassed: %s%b\n" "$GREEN" "$PASS" "$RESET"
printf "  %bFailed: %s%b\n" "$RED" "$FAIL" "$RESET"
printf "  %bSkipped: %s%b\n" "$YELLOW" "$SKIP" "$RESET"

if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
