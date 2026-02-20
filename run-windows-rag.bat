@echo off
REM ============================================================================
REM Windows Run Script for bchat with RAG (LanceDB) Support
REM ============================================================================
REM This script runs the bchat application with LanceDB RAG enabled on Windows
REM 
REM Prerequisites:
REM   1. Built with build-windows-rag.bat
REM   2. LanceDB DLL in PATH (optional, for better performance)
REM
REM Usage:
REM   run-windows-rag.bat
REM ============================================================================

setlocal enabledelayedexpansion

echo ================================================================
echo Windows RAG Run Script for bchat
echo ================================================================
echo.

REM Check if the executable exists
if not exist "build\memos.exe" (
    echo [ERROR] build\memos.exe not found
    echo Please run build-windows-rag.bat first
    exit /b 1
)

REM Set environment variables for RAG
set RAG_PIPELINE_ENABLED=true
set LANCEDB_STORAGE_PROVIDER=local
set LANCEDB_LOCAL_PATH=build\data\lancedb

REM Set embedding model (you can change this)
set EMBEDDING_MODEL=qwen/qwen3-embedding-8b
set EMBEDDING_BATCH_SIZE=32

REM Add LanceDB DLL to PATH if it exists
set LANCEDB_LIB_DIR=lib\windows_amd64
if exist "%LANCEDB_LIB_DIR%\lancedb_go.dll" (
    set PATH=%LANCEDB_LIB_DIR%;%PATH%
    echo [INFO] Added lancedb_go.dll to PATH
)

echo Configuration:
echo   RAG_PIPELINE_ENABLED=%RAG_PIPELINE_ENABLED%
echo   LANCEDB_STORAGE_PROVIDER=%LANCEDB_STORAGE_PROVIDER%
echo   LANCEDB_LOCAL_PATH=%LANCEDB_LOCAL_PATH%
echo   EMBEDDING_MODEL=%EMBEDDING_MODEL%
echo.

echo ================================================================
echo Starting bchat with RAG support...
echo ================================================================
echo.

REM Run the application
build\memos.exe --mode dev --data build\data

exit /b %errorlevel%
