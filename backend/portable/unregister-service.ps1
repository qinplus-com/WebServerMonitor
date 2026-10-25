﻿﻿﻿# ============================================================
# 解压版 - 卸载系统服务（需管理员，由 卸载系统服务.bat 自动提权调用）
# ============================================================
function Run-Sc($argstr) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = Join-Path $env:WINDIR 'System32\sc.exe'
    $psi.Arguments = $argstr
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $p = [System.Diagnostics.Process]::Start($psi)
    $p.StandardOutput.ReadToEnd() | Write-Host
    $p.WaitForExit()
}

if (-not (Get-Service QinJiaGateway -ErrorAction SilentlyContinue)) {
    Write-Host '未找到 QinJiaGateway 服务，无需卸载。'
    Start-Sleep -Seconds 3
    exit
}

Run-Sc 'stop QinJiaGateway'
Start-Sleep -Seconds 2
Run-Sc 'delete QinJiaGateway'

# 移除防火墙规则（程序规则 + 端口规则），数据文件保留
$fw = Join-Path $env:WINDIR 'System32\netsh.exe'
& $fw advfirewall firewall delete rule name='QinJiaGateway'  | Out-Null
& $fw advfirewall firewall delete rule name='QinJiaGateway_app' | Out-Null

Write-Host ''
Write-Host '系统服务与防火墙规则已卸载。数据文件仍保留在 data 目录中。'
Start-Sleep -Seconds 3
