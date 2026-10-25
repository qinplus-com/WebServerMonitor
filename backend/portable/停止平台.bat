@echo off
chcp 65001 >nul

tasklist /fi "imagename eq gateway.exe" | find /i "gateway.exe" >nul
if %errorlevel% neq 0 (
  echo 平台当前未运行。
  timeout /t 3 >nul
  exit /b
)

taskkill /f /im gateway.exe
echo 平台已停止。
timeout /t 2 >nul
exit /b
