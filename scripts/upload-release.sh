#!/bin/bash
# Upload binaries to GitHub Release

set -e

if [ $# -ne 1 ]; then
    echo "Usage: $0 <tag>"
    echo "Example: $0 v1.2.0"
    echo ""
    echo "Prerequisites:"
    echo "  - Build binaries must exist in ./dist/ directory"
    echo "  - GitHub CLI (gh) must be authenticated: gh auth login"
    echo "  - You must have permission to create releases"
    exit 1
fi

TAG=$1
VERSION=${TAG#v}
RELEASE_NOTES=""
echo "[run] Uploading binaries for release ${TAG}..."

for file in "docs/releases/RELEASE-NOTES-v${VERSION}.md" "docs/releases/RELEASE-NOTES-${VERSION}.md" "docs/releases/RELEASE-NOTES.md"; do
    if [ -f "$file" ]; then
        RELEASE_NOTES=$file
        break
    fi
done

# Check if dist directory exists
if [ ! -d "dist" ]; then
    echo "[error] dist/ directory not found. Run build-all.sh first."
    exit 1
fi

# Check if release exists
echo "[check] Checking if release ${TAG} exists..."
if ! gh release view "${TAG}" > /dev/null 2>&1; then
    echo "[error] Release ${TAG} does not exist. Create it first:"
    if [ -n "$RELEASE_NOTES" ]; then
        echo "   gh release create ${TAG} -t \"${TAG}\" -F ${RELEASE_NOTES}"
    else
        echo "   gh release create ${TAG} -t \"${TAG}\" --generate-notes"
    fi
    exit 1
fi

# Find all binary files
echo "[find] Found the following release artifacts:"
find dist/ -type f \( -name "*.tar.gz" -o -name "*.zip" -o -name "SHA256SUMS" \) | sort

echo ""
read -p "Do you want to upload these files? (y/n) " -r
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "[error] Cancelled."
    exit 1
fi

# Upload files
echo "[upload] Uploading files to release ${TAG}..."
for file in dist/*.tar.gz dist/*.zip dist/SHA256SUMS; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        echo "  [upload] Uploading ${filename}..."
        gh release upload "${TAG}" "$file"
        if [ $? -eq 0 ]; then
            echo "  [ok] Uploaded ${filename}"
        else
            echo "  [error] Failed to upload ${filename}"
            exit 1
        fi
    fi
done

echo ""
echo "[ok] All files uploaded successfully!"
echo "[package] Release URL: https://github.com/owner/repo/releases/tag/${TAG}"
