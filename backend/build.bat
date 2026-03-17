@echo off
set "GOROOT=D:\Program Files\Go"
set "PATH=D:\Program Files\Go\bin;%PATH%"
cd /d "F:\output\AirShare\backend"
go build ./... 2>&1
if %ERRORLEVEL% EQU 0 (
    echo Build successful!
) else (
    echo Build failed!
)
