@echo off
echo Building Windows executable...

go build -o lsm.exe main.go

if %errorlevel% neq 0 (
    echo Build failed!
    exit /b %errorlevel%
)

echo Success! Binary created as lsm.exe
