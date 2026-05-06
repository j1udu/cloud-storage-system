#!/usr/bin/env bash
# Cloud Storage Dev Startup Script (Linux / Mac)
# Usage: bash dev.sh
set -euo pipefail

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { echo -e "${GREEN}[INFO]${RESET} $1"; }
warn()  { echo -e "${YELLOW}[WARN]${RESET} $1"; }
error() { echo -e "${RED}[ERROR]${RESET} $1"; }

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKEND_PID=""
FRONTEND_PID=""

cleanup() {
    echo ""
    info "Stopping services..."
    [ -n "$BACKEND_PID" ]  && kill "$BACKEND_PID"  2>/dev/null && info "Backend stopped"
    [ -n "$FRONTEND_PID" ] && kill "$FRONTEND_PID" 2>/dev/null && info "Frontend stopped"
    info "Docker containers still running (stop with: docker compose down)"
    exit 0
}
trap cleanup INT TERM

# ── Check Dependencies ──
echo -e "\n${BOLD}${CYAN}=== Check Dependencies ===${RESET}\n"

if ! command -v docker &>/dev/null; then
    error "Docker not found"
    echo "  Install: https://www.docker.com/products/docker-desktop/"
    echo "  Linux:   curl -fsSL https://get.docker.com | sh"
    exit 1
fi
info "Docker $(docker --version | grep -oE '[0-9]+\.[0-9]+\.[0-9]+')"

if ! command -v go &>/dev/null; then
    warn "Go not found, installing..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install go
    else
        wget -qO- https://go.dev/dl/go1.24.3.linux-amd64.tar.gz | sudo tar -C /usr/local -xzf -
        export PATH=$PATH:/usr/local/go/bin
    fi
    command -v go &>/dev/null || { error "Go install failed: https://go.dev/dl/"; exit 1; }
fi
info "Go $(go version | grep -oE 'go[0-9]+\.[0-9]+\.[0-9]+')"

if ! command -v npm &>/dev/null; then
    warn "Node not found, installing..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install node
    else
        curl -fsSL https://deb.nodesource.com/setup_lts.x | sudo -E bash -
        sudo apt-get install -y nodejs
    fi
    command -v npm &>/dev/null || { error "Node install failed: https://nodejs.org/"; exit 1; }
fi
info "Node $(node --version)"

# ── Start Infrastructure ──
echo -e "\n${BOLD}${CYAN}=== Start Infrastructure ===${RESET}\n"

cd "$ROOT_DIR"

if docker compose ps -q 2>/dev/null | grep -q .; then
    info "Docker containers already running"
else
    info "Starting MySQL / Redis / MinIO..."
    docker compose up -d
fi

info "Waiting for services..."
for svc in mysql redis minio; do
    i=0
    while [ $i -lt 60 ]; do
        status=$(docker inspect --format='{{.State.Health.Status}}' "$(docker compose ps -q $svc 2>/dev/null)" 2>/dev/null || echo "unknown")
        [ "$status" = "healthy" ] && break
        i=$((i + 2))
        sleep 2
    done
    if [ "$status" = "healthy" ]; then
        info "$svc ready"
    else
        error "$svc not ready after 60s"
        exit 1
    fi
done

# ── Start Backend ──
echo -e "\n${BOLD}${CYAN}=== Start Backend ===${RESET}\n"

cd "$ROOT_DIR/backend"
go mod download
info "Compiling and starting backend..."
go run cmd/server/main.go &
BACKEND_PID=$!
cd "$ROOT_DIR"

i=0
while [ $i -lt 60 ]; do
    curl -sf http://localhost:8080/ping >/dev/null 2>&1 && break
    i=$((i + 2))
    sleep 2
done
if curl -sf http://localhost:8080/ping >/dev/null 2>&1; then
    info "Backend ready (PID: $BACKEND_PID)"
else
    error "Backend failed to start"
    cleanup
fi

# ── Start Frontend ──
echo -e "\n${BOLD}${CYAN}=== Start Frontend ===${RESET}\n"

cd "$ROOT_DIR/frontend"
npm install --silent 2>/dev/null
info "Starting frontend..."
npm run dev &
FRONTEND_PID=$!
cd "$ROOT_DIR"

i=0
while [ $i -lt 30 ]; do
    curl -sf http://localhost:5173 >/dev/null 2>&1 && break
    i=$((i + 2))
    sleep 2
done
if curl -sf http://localhost:5173 >/dev/null 2>&1; then
    info "Frontend ready (PID: $FRONTEND_PID)"
else
    warn "Frontend may still be starting, visit later"
fi

# ── Summary ──
echo -e "\n${BOLD}==============================${RESET}"
echo -e "${GREEN}  Cloud Storage Started!${RESET}"
echo ""
echo -e "  Frontend:  ${CYAN}http://localhost:5173${RESET}"
echo -e "  Backend:   ${CYAN}http://localhost:8080${RESET}"
echo -e "  MinIO:     ${CYAN}http://localhost:9001${RESET} (minioadmin / minioadmin123)"
echo ""
echo "  Press ${BOLD}Ctrl+C${RESET} to stop (Docker keeps running)"
echo -e "${BOLD}==============================${RESET}"

wait
