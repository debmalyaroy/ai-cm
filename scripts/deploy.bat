@echo off
REM AI-CM Deploy Script (Windows)
REM Usage: deploy.bat [dev|prod]
setlocal enabledelayedexpansion

set "ROOT_DIR=%~dp0.."
set "ENV=%~1"
if "%ENV%"=="" set "ENV=dev"

if "%ENV%"=="dev" goto :dev
if "%ENV%"=="prod" goto :prod
goto :usage

:dev
echo [INFO] Starting development environment...
cd /d "%ROOT_DIR%"

echo [INFO] Starting Docker Compose stack (database)...
docker compose up -d

echo [INFO] Waiting for PostgreSQL...
timeout /t 5 /nobreak >nul

echo [INFO] Starting backend...
start "AI-CM Backend" /D "%ROOT_DIR%\src\backend" cmd /c "go run ./cmd/server"

echo [INFO] Starting frontend...
start "AI-CM Frontend" /D "%ROOT_DIR%\src\apps\web" cmd /c "npm run dev"

echo.
echo [INFO] Development environment ready!
echo   Frontend: http://localhost:3000
echo   Backend:  http://localhost:8080
echo   Login:    http://localhost:3000/login (admin/admin)
echo.
echo Close the terminal windows to stop services.
goto :done

:prod
echo [INFO] Building production images...
cd /d "%ROOT_DIR%"

call "%~dp0build.bat" all

echo [INFO] Starting production stack...
cd /d "%ROOT_DIR%\infra"
docker compose -f docker-compose.prod.yml up -d --build

echo.
echo [INFO] Production deployment complete!
echo   Application: http://localhost
echo   API:         http://localhost:8080
goto :done

:usage
echo Usage: deploy.bat [dev^|prod]
exit /b 1

:done
endlocal
