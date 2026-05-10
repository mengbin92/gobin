#!/bin/bash
# Build script for Gobin - cross-platform compilation

set -e

# Set version from git tag or use default
VERSION=${VERSION:-$(git describe --tags --exact-match 2>/dev/null || echo "dev")}
COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE=$(date -u '+%Y-%m-%d_%H:%M:%S')

# Create output directory
mkdir -p dist
rm -rf dist/*

echo "[build] Building Gobin ${VERSION} (commit: ${COMMIT})"
echo "[date] Build date: ${BUILD_DATE}"
echo ""

# Build function
build_binary() {
    local GOOS=$1
    local GOARCH=$2
    local OUTPUT_NAME=$3

    echo "[package] Building for ${GOOS}/${GOARCH}..."

    env GOOS=${GOOS} GOARCH=${GOARCH} \
        go build \
        -ldflags "-s -w -X 'github.com/mengbin92/gobin/cmd/gobin/commands.Version=${VERSION}' -X 'github.com/mengbin92/gobin/cmd/gobin/commands.Commit=${COMMIT}' -X 'github.com/mengbin92/gobin/cmd/gobin/commands.BuildDate=${BUILD_DATE}'" \
        -o "dist/${OUTPUT_NAME}" \
        ./cmd/gobin

    if [ $? -eq 0 ]; then
        echo "[ok] Success: ${OUTPUT_NAME}"
        # Make executable on Unix-like systems
        if [[ "${GOOS}" != "windows" ]]; then
            chmod +x "dist/${OUTPUT_NAME}"
        fi
        # Show file info
        ls -lh "dist/${OUTPUT_NAME}"
    else
        echo "[error] Failed: ${OUTPUT_NAME}"
        exit 1
    fi
    echo ""
}

# Linux
build_binary "linux" "amd64" "gobin-v${VERSION}-linux-amd64"
build_binary "linux" "arm64" "gobin-v${VERSION}-linux-arm64"

# macOS
build_binary "darwin" "amd64" "gobin-v${VERSION}-darwin-amd64"
build_binary "darwin" "arm64" "gobin-v${VERSION}-darwin-arm64"

# Windows
build_binary "windows" "amd64" "gobin-v${VERSION}-windows-amd64.exe"

# FreeBSD (optional)
build_binary "freebsd" "amd64" "gobin-v${VERSION}-freebsd-amd64"

# Optional: Build for more platforms
# build_binary "openbsd" "amd64" "gobin-${VERSION}-openbsd-amd64"
# build_binary "netbsd" "amd64" "gobin-${VERSION}-netbsd-amd64"

echo "[ok] All builds completed successfully!"
echo ""
echo "[dir] Artifacts located in: ./dist/"
ls -lh dist/
