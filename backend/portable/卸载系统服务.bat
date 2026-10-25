@echo off
chcp 65001 >nul
cd /d %~dp0

rem 检测管理员权限，无权限则自动 UAC 提权重启本脚本
net session >nul 2>&1
if %errorlevel% neq 0 (
  echo Requesting administrator privileges...
  powershell -NoProfile -Command "Start-Process '%~f0' -Verb RunAs"
  exit /b
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0unregister-service.ps1"
