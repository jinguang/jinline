#!/usr/bin/env bash
# jinline 安装脚本（AlmaLinux 9.x）
# 用法：sudo ./deploy/install.sh
# 可选环境变量：INSTALL_DIR=/opt/jinline DATA_DIR=/var/lib/jinline SERVICE_USER=jinline
set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "错误：请使用 root 或 sudo 运行本脚本。" >&2
  exit 1
fi

DIST_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_DIR="${INSTALL_DIR:-/opt/jinline}"
DATA_DIR="${DATA_DIR:-/var/lib/jinline}"
SERVICE_USER="${SERVICE_USER:-jinline}"
SERVICE_NAME="jinline"

need_file() {
  if [[ ! -f "$1" ]]; then
    echo "错误：缺少文件 $1，请在完整的 dist 目录中运行本脚本。" >&2
    exit 1
  fi
}

need_file "${DIST_ROOT}/jinline"
need_file "${DIST_ROOT}/web/index.html"
need_file "${DIST_ROOT}/configs/config.yaml"
need_file "${DIST_ROOT}/deploy/jinline.service"
need_file "${DIST_ROOT}/deploy/schema.sql"

if ! command -v systemctl >/dev/null 2>&1; then
  echo "错误：未检测到 systemd，当前系统不受支持。" >&2
  exit 1
fi

echo "==> 创建服务用户 ${SERVICE_USER}"
if ! id -u "${SERVICE_USER}" >/dev/null 2>&1; then
  useradd --system --home-dir "${DATA_DIR}" --shell /sbin/nologin "${SERVICE_USER}"
fi

echo "==> 创建安装目录 ${INSTALL_DIR} 与数据目录 ${DATA_DIR}"
install -d -m 0755 "${INSTALL_DIR}"
install -d -o "${SERVICE_USER}" -g "${SERVICE_USER}" -m 0750 "${DATA_DIR}"

echo "==> 安装程序与静态文件"
install -m 0755 "${DIST_ROOT}/jinline" "${INSTALL_DIR}/jinline"
if [[ -f "${DIST_ROOT}/VERSION" ]]; then
  install -m 0644 "${DIST_ROOT}/VERSION" "${INSTALL_DIR}/VERSION"
fi
rm -rf "${INSTALL_DIR}/web"
cp -a "${DIST_ROOT}/web" "${INSTALL_DIR}/web"
mkdir -p "${INSTALL_DIR}/examples" "${INSTALL_DIR}/docs" "${INSTALL_DIR}/configs" "${INSTALL_DIR}/deploy"
cp -a "${DIST_ROOT}/examples/." "${INSTALL_DIR}/examples/"
cp -a "${DIST_ROOT}/docs/." "${INSTALL_DIR}/docs/"
install -m 0644 "${DIST_ROOT}/deploy/schema.sql" "${INSTALL_DIR}/deploy/schema.sql"

if [[ -f "${INSTALL_DIR}/configs/config.yaml" ]]; then
  echo "==> 检测到已有配置，保留 ${INSTALL_DIR}/configs/config.yaml"
else
  echo "==> 写入配置模板（请务必修改数据库与 Redis 密码）"
  install -m 0640 -o root -g "${SERVICE_USER}" \
    "${DIST_ROOT}/configs/config.yaml" "${INSTALL_DIR}/configs/config.yaml"
fi

echo "==> 安装 systemd 单元"
install -m 0644 "${DIST_ROOT}/deploy/jinline.service" "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}.service" >/dev/null

echo
echo "安装完成。后续步骤："
echo "  1. 编辑 ${INSTALL_DIR}/configs/config.yaml，填写 MySQL / Redis 密码"
echo "  2. 确认 MySQL 已创建 jinline 库与账号，Redis 密码与配置一致"
echo "  3. systemctl start ${SERVICE_NAME}"
echo "  4. systemctl status ${SERVICE_NAME} && curl http://127.0.0.1:8080/api/health"
