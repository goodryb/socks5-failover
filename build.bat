@echo off
REM 本地构建脚本：产物统一输出到 dist\
REM   build.bat        仅构建当前平台，产出 dist\socks5-failover.exe（与 start.vbs 对应）
REM   build.bat all    交叉编译 5 个平台，产出 dist\socks5-failover-<os>-<arch>[.exe]
setlocal enabledelayedexpansion
cd /d "%~dp0"

set "CGO_ENABLED=0"

echo [1/3] go vet
go vet ./... || exit /b 1

if not exist dist mkdir dist

if /i "%~1"=="all" (
    echo [2/3] 交叉编译 5 个平台
    for %%t in (linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64) do (
        for /f "tokens=1,2 delims=/" %%o in ("%%t") do (
            set "TOS=%%o"
            set "TAR=%%p"
            set "TEXT="
            if /i "!TOS!"=="windows" set "TEXT=.exe"
            echo       build !TOS!/!TAR!
            set "GOOS=!TOS!"
            set "GOARCH=!TAR!"
            go build -trimpath -ldflags "-s -w" -o "dist\socks5-failover-!TOS!-!TAR!!TEXT!" . || exit /b 1
        )
    )
) else (
    for /f "delims=" %%o in ('go env GOOS') do set "LOS=%%o"
    for /f "delims=" %%a in ('go env GOARCH') do set "LAR=%%a"
    set "LEXT="
    if /i "!LOS!"=="windows" set "LEXT=.exe"
    echo [2/3] 构建当前平台 !LOS!/!LAR!
    go build -trimpath -ldflags "-s -w" -o "dist\socks5-failover!LEXT!" . || exit /b 1
)

echo [3/3] dist 产物
for %%f in (dist\*) do echo       %%~nxf  %%~zf bytes

endlocal
