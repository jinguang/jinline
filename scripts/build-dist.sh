#!/usr/bin/env bash
# 构建可部署产物到 dist/，默认目标平台 linux/amd64。
# 用法：
#   bash scripts/build-dist.sh
#   GOARCH=arm64 bash scripts/build-dist.sh
#   VERSION=1.2.0 bash scripts/build-dist.sh
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="${ROOT_DIR}/dist"

GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"
VERSION="${VERSION:-$(git -C "${ROOT_DIR}" describe --tags --always --dirty 2>/dev/null || echo "dev")}"
GIT_COMMIT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD 2>/dev/null || echo "unknown")"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "==> [1/3] 构建前端"
(cd "${ROOT_DIR}/frontend" && npm ci --no-fund --no-audit && npm run build)

echo "==> [2/3] 编译后端 (${GOOS}/${GOARCH}, version=${VERSION})"
mkdir -p "${DIST_DIR}"
(cd "${ROOT_DIR}/backend" && \
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
  go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" \
  -o "${DIST_DIR}/jinline" .)

echo "==> [3/3] 组装 dist"
rm -rf "${DIST_DIR}/web" "${DIST_DIR}/configs" "${DIST_DIR}/deploy" "${DIST_DIR}/examples" "${DIST_DIR}/docs"
mkdir -p "${DIST_DIR}/configs" "${DIST_DIR}/web" "${DIST_DIR}/deploy" "${DIST_DIR}/examples" "${DIST_DIR}/docs"

cp -a "${ROOT_DIR}/frontend/dist/." "${DIST_DIR}/web/"
cp -a "${ROOT_DIR}/deploy/install.sh" "${ROOT_DIR}/deploy/jinline.service" "${ROOT_DIR}/deploy/schema.sql" "${DIST_DIR}/deploy/"
cp -a "${ROOT_DIR}/deploy/config.prod.yaml" "${DIST_DIR}/configs/config.yaml"
cp -a "${ROOT_DIR}/examples/." "${DIST_DIR}/examples/"
cp -a "${ROOT_DIR}/docs/." "${DIST_DIR}/docs/"

chmod +x "${DIST_DIR}/jinline" "${DIST_DIR}/deploy/install.sh"

cat > "${DIST_DIR}/VERSION" <<EOF
version: ${VERSION}
commit: ${GIT_COMMIT}
build_time: ${BUILD_TIME}
target: ${GOOS}/${GOARCH}
EOF

echo "==> 构建完成：${DIST_DIR}"
