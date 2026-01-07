#!/bin/bash
# Upload binaries to GitHub Release

set -e

if [ $# -ne 1 ]; then
    echo "Usage: $0 <tag>"
    echo "Example: $0 v1.0.0"
    echo ""
    echo "Prerequisites:"
    echo "  - Build binaries must exist in ./dist/ directory"
    echo "  - GitHub CLI (gh) must be authenticated: gh auth login"
    echo "  - You must have permission to create releases"
    exit 1
fi

TAG=$1
echo "🚀 Uploading binaries for release ${TAG}..."

# Check if dist directory exists
if [ ! -d "dist" ]; then
    echo "❌ dist/ directory not found. Run build-all.sh first."
    exit 1
fi

# Check if release exists
echo "📋 Checking if release ${TAG} exists..."
if ! gh release view "${TAG}" > /dev/null 2>&1; then
    echo "❌ Release ${TAG} does not exist. Create it first:"
    echo "   gh release create ${TAG} -t \"${TAG}\" -F RELEASE-NOTES-v1.0.md"
    exit 1
fi

# Find all binary files
echo "🔍 Found the following binaries:"
find dist/ -type f -name "gobin-*" | sort

echo ""
read -p "Do you want to upload these files? (y/n) " -r
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "❌ Cancelled."
    exit 1
fi

# Upload files
echo "⏫ Uploading files to release ${TAG}..."
for file in dist/*; do
    if [ -f "$file" ]; then
        filename=$(basename "$file")
        echo "  📤 Uploading ${filename}..."
        gh release upload "${TAG}" "$file"
        if [ $? -eq 0 ]; then
            echo "  ✅ Uploaded ${filename}"
        else
            echo "  ❌ Failed to upload ${filename}"
            exit 1
        fi
    fi
done

echo ""
echo "🎉 All files uploaded successfully!"
echo "📦 Release URL: https://github.com/owner/repo/releases/tag/${TAG}"
