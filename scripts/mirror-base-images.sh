#!/bin/bash
# Mirror Docker Hub base images to GHCR
# This eliminates Docker Hub rate limit issues by hosting our own copies
#
# Prerequisites:
#   docker login ghcr.io -u <username> -p <PAT>
#
# Usage:
#   ./scripts/mirror-base-images.sh

set -euo pipefail

REGISTRY="ghcr.io/madfam-org/base"

# Images to mirror (Docker Hub -> GHCR)
IMAGES=(
  "node:20-alpine"
  "nginx:alpine"
  "golang:1.24-alpine"
  "alpine:3.20"
  "alpine:3.19"
  "docker:27-dind"
)

echo "🚂 Mirroring Docker Hub images to GHCR..."
echo "   Target registry: $REGISTRY"
echo ""

for img in "${IMAGES[@]}"; do
  name=$(echo "$img" | cut -d: -f1)
  tag=$(echo "$img" | cut -d: -f2)
  target="$REGISTRY/$name:$tag"

  echo "📦 Processing: $img -> $target"

  # Pull from Docker Hub
  echo "   Pulling from Docker Hub..."
  docker pull "$img"

  # Tag for GHCR
  docker tag "$img" "$target"

  # Push to GHCR
  echo "   Pushing to GHCR..."
  docker push "$target"

  echo "   ✅ Done: $target"
  echo ""
done

echo "🎉 All images mirrored successfully!"
echo ""
echo "Images available at:"
for img in "${IMAGES[@]}"; do
  name=$(echo "$img" | cut -d: -f1)
  tag=$(echo "$img" | cut -d: -f2)
  echo "  - $REGISTRY/$name:$tag"
done
