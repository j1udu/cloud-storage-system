@echo off
title cloud-storage-dev

echo.
echo === Check Dependencies ===
echo.

where docker >nul 2>&1 || (
    echo [ERROR] Docker not found, install: https://www.docker.com/products/docker-desktop/
    pause
    exit /b 1
)
echo [INFO] Docker OK

where go >nul 2>&1 || (
    echo [ERROR] Go not found, install: https://go.dev/dl/
    pause
    exit /b 1
)
echo [INFO] Go OK

where npm >nul 2>&1 || (
    echo [ERROR] Node not found, install: https://nodejs.org/
    pause
    exit /b 1
)
echo [INFO] Node OK

echo.
echo === Start Infrastructure ===
echo.

docker compose up -d

echo [INFO] Waiting for services...
timeout /t 10 /nobreak >nul

echo.
echo === Start Backend ===
echo.

cd /d "%~dp0backend"
start "backend" cmd /k "go run cmd/server/main.go"
cd /d "%~dp0"

echo [INFO] Waiting for backend to compile...
timeout /t 15 /nobreak >nul

echo.
echo === Start Frontend ===
echo.

cd /d "%~dp0frontend"
start "frontend" cmd /k "npm install && npm run dev"
cd /d "%~dp0"

echo.
echo ==============================
echo   Cloud Storage Started!
echo.
echo   Frontend:  http://localhost:5173
echo   Backend:   http://localhost:8080
echo   MinIO:     http://localhost:9001
echo.
echo   Close this window won't stop services
echo   Stop all:  docker compose down
echo ==============================
echo.
pause
