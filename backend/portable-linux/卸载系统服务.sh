#!/bin/bash
# WebServerMonitor 社区版 - 卸载 systemd 系统服务（不删除数据目录）
set -e
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 sudo 运行：sudo $0"
  exit 1
fi
systemctl disable --now webservermonitor.service 2>/dev/null || true
rm -f /etc/systemd/system/webservermonitor.service
systemctl daemon-reload
echo "服务已卸载（数据目录保留，如需彻底清理请手动删除安装目录）"
