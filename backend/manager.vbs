' ============================================================
' 平台管理工具 - 无控制台启动器
' 由快捷方式调用 wscript.exe 执行本脚本，避免出现命令行窗口
' ============================================================
Option Explicit
Dim fso, sh, sDir
Set fso = CreateObject("Scripting.FileSystemObject")
Set sh = CreateObject("Shell.Application")
sDir = fso.GetParentFolderName(WScript.ScriptFullName)
' runas = 以管理员身份运行（触发一次 UAC 确认）；末参数 0 = 窗口从创建即隐藏
sh.ShellExecute "powershell.exe", _
    "-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File """ & sDir & "\manager.ps1""", _
    sDir, "runas", 0
