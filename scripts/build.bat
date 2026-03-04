@echo off
REM AI-CM Build Script (Windows)
REM Usage: build.bat [all|backend|frontend|docker|test]
setlocal enabledelayedexpansion

set "ROOT_DIR=%~dp0.."
set "TARGET=%~1"
if "%TARGET%"=="" set "TARGET=all"

goto :%TARGET% 2>nul || goto :usage

:backend
echo [INFO] Building backend...
cd /d "%ROOT_DIR%\src\backend"

echo [INFO] Running tests...
go test ./... -count=1
if errorlevel 1 (
    echo [ERROR] Backend tests failed
    exit /b 1
)

echo [INFO] Building binary...
set CGO_ENABLED=0
go build -o "%ROOT_DIR%\bin\aicm-server.exe" ./cmd/server
if errorlevel 1 (
    echo [ERROR] Backend build failed
    exit /b 1
)

echo [INFO] Backend built: bin\aicm-server.exe
if "%TARGET%"=="all" goto :frontend
goto :done

:frontend
echo [INFO] Building frontend...
cd /d "%ROOT_DIR%\src\apps\web"

if not exist "node_modules" (
    echo [INFO] Installing dependencies...
    call npm install
)

echo [INFO] Building Next.js app...
call npm run build
if errorlevel 1 (
    echo [ERROR] Frontend build failed
    exit /b 1
)

echo [INFO] Frontend built: src\apps\web\.next\
goto :done

:docker
echo [INFO] Building Docker images...
cd /d "%ROOT_DIR%"
docker compose build
if errorlevel 1 (
    echo [ERROR] Docker build failed
    exit /b 1
)
echo [INFO] Docker images built successfully
goto :done

:test
echo [INFO] Running all backend tests...
cd /d "%ROOT_DIR%\src\backend"
go test ./... -v -count=1 -cover
goto :done

:all
call :backend
call :frontend
echo [INFO] All builds complete!
goto :done

:usage
echo Usage: build.bat [all^|backend^|frontend^|docker^|test]
exit /b 1

:done
endlocal
