#!/bin/bash
# WebServerMonitor 社区版 - 停止平台
pkill -f "QinJiaGateway|WebServerMonitor" 2>/dev/null
# 退化兜底：按可执行文件名匹配当前目录下的 gateway 进程
pkill -x gateway 2>/dev/null
echo "已发送停止信号"
