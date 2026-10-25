#!/bin/bash
# WebServerMonitor 社区版 - 注册 systemd 系统服务（开机自启 + 崩溃自动重启）
# 用法：sudo ./注册系统服务.sh [安装目录]，默认当前目录
set -e
if [ "$(id -u)" -ne 0 ]; then
  echo "请使用 sudo 运行：sudo $0"
  exit 1
fi
DIR="${1:-$(cd "$(dirname "$0")" && pwd)}"
DIR="$(cd "$DIR" && pwd)"
chmod +x "$DIR/gateway"
cat > /etc/systemd/system/webservermonitor.service <<EOF
[Unit]
Description=WebServerMonitor Community (QinJia Web Gateway)
After=network.target

[Service]
Type=simple
WorkingDirectory=$DIR
ExecStart=$DIR/gateway
Restart=always
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
systemctl daemon-reload
systemctl enable --now webservermonitor.service
echo "服务已注册并启动：systemctl status webservermonitor"
echo "浏览器访问 http://$(hostname -I | awk '{print $1}'):8080/（默认 admin / admin123）"
