@echo off
rem ==============================================================================
rem Build script for Downloader (Windows CMD / Batch)
rem ==============================================================================

setlocal enabledelayedexpansion

set "APP_NAME=downloader"
set "PACKAGE=./cmd/downloader"
set "OUTPUT_DIR=bin"
set "LDFLAGS=-s -w"

set "TARGET=%~1"
if "%TARGET%"=="" set "TARGET=current"

if /i "%TARGET%"=="help" goto :usage
if /i "%TARGET%"=="-h" goto :usage
if /i "%TARGET%"=="--help" goto :usage
if /i "%TARGET%"=="clean" goto :clean

where go >nul 2>nul
if %ERRORLEVEL% neq 0 (
    echo [ERROR] 'go' command not found. Please install Go from https://golang.org/dl/ or add it to PATH.
    exit /b 1
)

if /i "%TARGET%"=="all" goto :build_all
if /i "%TARGET%"=="linux" goto :build_linux
if /i "%TARGET%"=="lunix" goto :build_linux
if /i "%TARGET%"=="windows" goto :build_windows
if /i "%TARGET%"=="win" goto :build_windows
if /i "%TARGET%"=="current" goto :build_current

echo [ERROR] Unknown target: %TARGET%
goto :usage

:usage
echo Usage: build.bat [TARGET]
echo.
echo Targets:
echo   all       Build binaries for both Linux and Windows (amd64 and arm64)
echo   linux     Build Linux binaries (amd64 and arm64)
echo   windows   Build Windows binaries (amd64 and arm64)
echo   current   Build binary for the current OS/architecture (default)
echo   clean     Remove built binaries in '%OUTPUT_DIR%\'
echo   help      Show this help message
echo.
exit /b 0

:clean
echo Cleaning output directory '%OUTPUT_DIR%'...
if exist "%OUTPUT_DIR%" (
    rmdir /s /q "%OUTPUT_DIR%"
)
echo Clean complete.
exit /b 0

:build_current
echo === Building current host target ===
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"
for /f "tokens=*" %%i in ('go env GOOS') do set "HOST_OS=%%i"
for /f "tokens=*" %%i in ('go env GOARCH') do set "HOST_ARCH=%%i"

set "EXT="
if "%HOST_OS%"=="windows" set "EXT=.exe"
set "OUTPUT_FILE=%OUTPUT_DIR%\%APP_NAME%%EXT%"

echo Building %APP_NAME% for host platform (%HOST_OS%/%HOST_ARCH%) -^> %OUTPUT_FILE%...
set "CGO_ENABLED=0"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_FILE%" %PACKAGE%
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Build failed!
    exit /b 1
)
echo === Build finished successfully! Output in '%OUTPUT_DIR%\' ===
exit /b 0

:build_linux
echo === Building Linux targets ===
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

echo Building %APP_NAME% for linux/amd64 -^> %OUTPUT_DIR%\%APP_NAME%-linux-amd64...
set "CGO_ENABLED=0"
set "GOOS=linux"
set "GOARCH=amd64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-linux-amd64" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1

echo Building %APP_NAME% for linux/arm64 -^> %OUTPUT_DIR%\%APP_NAME%-linux-arm64...
set "CGO_ENABLED=0"
set "GOOS=linux"
set "GOARCH=arm64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-linux-arm64" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1

copy /y "%OUTPUT_DIR%\%APP_NAME%-linux-amd64" "%OUTPUT_DIR%\%APP_NAME%" >nul 2>nul
echo === Build finished successfully! Outputs in '%OUTPUT_DIR%\' ===
exit /b 0

:build_windows
echo === Building Windows targets ===
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

echo Building %APP_NAME% for windows/amd64 -^> %OUTPUT_DIR%\%APP_NAME%-windows-amd64.exe...
set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=amd64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-windows-amd64.exe" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1

echo Building %APP_NAME% for windows/arm64 -^> %OUTPUT_DIR%\%APP_NAME%-windows-arm64.exe...
set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=arm64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-windows-arm64.exe" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1

copy /y "%OUTPUT_DIR%\%APP_NAME%-windows-amd64.exe" "%OUTPUT_DIR%\%APP_NAME%.exe" >nul 2>nul
echo === Build finished successfully! Outputs in '%OUTPUT_DIR%\' ===
exit /b 0

:build_all
echo === Building all targets (Linux ^& Windows) ===
if not exist "%OUTPUT_DIR%" mkdir "%OUTPUT_DIR%"

echo Building %APP_NAME% for linux/amd64...
set "CGO_ENABLED=0"
set "GOOS=linux"
set "GOARCH=amd64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-linux-amd64" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1

echo Building %APP_NAME% for linux/arm64...
set "CGO_ENABLED=0"
set "GOOS=linux"
set "GOARCH=arm64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-linux-arm64" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1
copy /y "%OUTPUT_DIR%\%APP_NAME%-linux-amd64" "%OUTPUT_DIR%\%APP_NAME%" >nul 2>nul

echo Building %APP_NAME% for windows/amd64...
set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=amd64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-windows-amd64.exe" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1

echo Building %APP_NAME% for windows/arm64...
set "CGO_ENABLED=0"
set "GOOS=windows"
set "GOARCH=arm64"
go build -trimpath -ldflags="%LDFLAGS%" -o "%OUTPUT_DIR%\%APP_NAME%-windows-arm64.exe" %PACKAGE%
if %ERRORLEVEL% neq 0 exit /b 1
copy /y "%OUTPUT_DIR%\%APP_NAME%-windows-amd64.exe" "%OUTPUT_DIR%\%APP_NAME%.exe" >nul 2>nul

echo === Build finished successfully! Outputs in '%OUTPUT_DIR%\' ===
exit /b 0
