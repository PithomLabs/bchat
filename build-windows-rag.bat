@echo off
REM ============================================================================
REM Windows Build Script for bchat with RAG (LanceDB) Support
REM ============================================================================
REM This script builds the bchat application with LanceDB RAG support on Windows
REM 
REM Prerequisites:
REM   1. Go with CGO support installed
REM   2. A C compiler (GCC via MSYS2 or TDM-GCC)
REM   3. PowerShell 5.1+
REM
REM Usage:
REM   1. Copy this script to your bchat project directory
REM   2. Run: build-windows-rag.bat
REM ============================================================================

setlocal enabledelayedexpansion

echo ================================================================
echo Windows RAG Build Script for bchat
echo ================================================================
echo.

REM Check if we're in the right directory
if not exist "Taskfilewin.yml" (
    echo [ERROR] Taskfilewin.yml not found. Please run this script from the bchat project directory.
    exit /b 1
)

if not exist "bin\memos\main.go" (
    echo [ERROR] bin\memos\main.go not found. Please run this script from the bchat project directory.
    exit /b 1
)

REM Check for Go
where go >nul 2>&1
if %errorlevel% neq 0 (
    echo [ERROR] Go is not installed or not in PATH
    echo Please install Go from https://go.dev/dl/
    exit /b 1
)

echo [1/5] Checking Go installation...
go version
echo.

REM Check for GCC (C compiler)
where gcc >nul 2>&1
if %errorlevel% neq 0 (
    echo [WARNING] GCC not found in PATH
    echo Please install a C compiler:
    echo   Option A: MSYS2 - https://www.msys2.org/
    echo     Run: pacman -S mingw-w64-x86_64-gcc
    echo   Option B: TDM-GCC - https://jmeubank.github.io/tdm-gcc/
    echo.
    echo The build may still work if using static libraries only.
)

echo [2/5] Downloading LanceDB Windows native libraries...
echo.

REM Create directories
if not exist "lib" mkdir lib
if not exist "include" mkdir include

REM Download LanceDB libraries using the PowerShell script
powershell -NoProfile -ExecutionPolicy Bypass -File "scripts\download-lancedb.ps1" v0.1.2

if %errorlevel% neq 0 (
    echo [ERROR] Failed to download LanceDB libraries
    exit /b 1
)

echo.
echo [3/5] Verifying downloaded libraries...

set LANCEDB_LIB_DIR=lib\windows_amd64

if not exist "%LANCEDB_LIB_DIR%\liblancedb_go.a" (
    echo [ERROR] liblancedb_go.a not found
    exit /b 1
)

echo   - liblancedb_go.a ... OK

if exist "%LANCEDB_LIB_DIR%\lancedb_go.dll" (
    echo   - lancedb_go.dll ... OK
) else (
    echo   - lancedb_go.dll ... NOT FOUND (optional, using static lib)
)

if not exist "include\lancedb.h" (
    echo [ERROR] lancedb.h not found
    exit /b 1
)

echo   - lancedb.h ... OK

echo.
echo [4/5] Building with RAG support...
echo.

REM Set CGO environment variables
set CGO_ENABLED=1
set CGO_CFLAGS=-Iinclude
set CGO_LDFLAGS=%LANCEDB_LIB_DIR%\liblancedb_go.a

REM Create build directory
if not exist "build" mkdir build

REM Build the application
go build -tags rag -o build\memos.exe .\bin\memos\main.go

if %errorlevel% neq 0 (
    echo [ERROR] Build failed
    exit /b 1
)

echo.
echo [5/5] Build complete!
echo.

if exist "build\memos.exe" (
    echo   Output: build\memos.exe
    for %%A in (build\memos.exe) do echo   Size: %%~zA bytes
)

echo.
echo ================================================================
echo Build successful!
echo ================================================================
echo.
echo To run with RAG enabled:
echo   set RAG_PIPELINE_ENABLED=true
echo   set LANCEDB_STORAGE_PROVIDER=local
echo   build\memos.exe --mode dev --data build\data
echo.

exit /b 0
