@echo off
chcp 65001 >nul
cd /d %~dp0

rem 检测平台是否已在运行
tasklist /fi "imagename eq gateway.exe" | find /i "gateway.exe" >nul
if %errorlevel% equ 0 (
  echo 平台已在运行，直接打开浏览器...
  start "" http://localhost:8080/
  exit /b
)

start "" "gateway.exe"
echo 正在启动平台，请稍候...
timeout /t 3 >nul
start "" http://localhost:8080/
exit /b
