#!/bin/bash
# WebServerMonitor 社区版 - 启动平台（前台运行，Ctrl+C 停止）
cd "$(dirname "$0")" || exit 1
chmod +x gateway 2>/dev/null
./gateway
