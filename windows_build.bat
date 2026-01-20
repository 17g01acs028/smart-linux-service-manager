@echo off
echo Building Linux executable (from Windows)...

REM Set environment variables for cross-compilation
set CGO_ENABLED=0
set GOOS=linux
set GOARCH=amd64

go build -o lsm-linux main.go

if %errorlevel% neq 0 (
    echo Build failed!
    exit /b %errorlevel%
)

echo Success! Binary created as lsm-linux
echo You can now upload it to your Linux server.
