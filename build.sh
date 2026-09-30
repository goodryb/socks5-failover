#!/usr/bin/env bash
# 本地构建脚本：产物统一输出到 dist/
#   ./build.sh        仅构建当前平台 -> dist/socks5-failover[.exe]
#   ./build.sh all    交叉编译 5 个平台 -> dist/socks5-failover-<os>-<arch>[.exe]
set -euo pipefail

cd "$(dirname "$0")"

BIN=socks5-failover
TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

export CGO_ENABLED=0
BUILD_FLAGS=(-trimpath -ldflags "-s -w")

echo "==> go vet"
go vet ./...

if [ "${1:-}" = "all" ]; then
    mkdir -p dist
    for t in $TARGETS; do
        os=${t%/*}
        arch=${t#*/}
        ext=""
        if [ "$os" = "windows" ]; then
            ext=".exe"
        fi
        out="dist/$BIN-$os-$arch$ext"
        echo "==> build $os/$arch -> $out"
        GOOS=$os GOARCH=$arch go build "${BUILD_FLAGS[@]}" -o "$out" .
    done
else
    os=$(go env GOOS)
    arch=$(go env GOARCH)
    ext=""
    if [ "$os" = "windows" ]; then
        ext=".exe"
    fi
    # 不带平台后缀，与 start.vbs 期望的 dist\socks5-failover.exe 保持一致
    out="dist/$BIN$ext"
    mkdir -p dist
    echo "==> build $os/$arch -> $out"
    go build "${BUILD_FLAGS[@]}" -o "$out" .
fi

echo "==> dist/ 内容"
ls -lh dist/
