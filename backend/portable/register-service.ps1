﻿﻿﻿# ============================================================
# 解压版 - 注册系统服务（需管理员，由 注册系统服务.bat 自动提权调用）
# ============================================================
$dir = Split-Path $MyInvocation.MyCommand.Path -Parent
$exe = Join-Path $dir 'gateway.exe'

# 用 ProcessStartInfo 精确传递参数（sc 的 binPath 内嵌引号必须 \" 转义）
function Run-Sc($argstr) {
    $psi = New-Object System.Diagnostics.ProcessStartInfo
    $psi.FileName = Join-Path $env:WINDIR 'System32\sc.exe'
    $psi.Arguments = $argstr
    $psi.UseShellExecute = $false
    $psi.RedirectStandardOutput = $true
    $p = [System.Diagnostics.Process]::Start($psi)
    $p.StandardOutput.ReadToEnd() | Write-Host
    $p.WaitForExit()
    return $p.ExitCode
}

if (Get-Service QinJiaGateway -ErrorAction SilentlyContinue) {
    Write-Host '服务 QinJiaGateway 已存在，先删除旧注册...'
    Run-Sc 'stop QinJiaGateway' | Out-Null
    Run-Sc 'delete QinJiaGateway' | Out-Null
    Start-Sleep -Seconds 1
}

$r1 = Run-Sc ('create QinJiaGateway binPath= "\"' + $exe + '\"" start= auto displayname= "勤加软件WEB服务器运维管理平台"')
Run-Sc 'description QinJiaGateway "勤加软件WEB服务器运维管理平台后台服务"' | Out-Null
Run-Sc 'failure QinJiaGateway reset= 60 actions= restart/5000/restart/5000/restart/5000' | Out-Null
Run-Sc 'start QinJiaGateway' | Out-Null

# 防火墙放行：程序规则（出现在「允许的应用」列表）+ 端口规则（放行 TCP 端口）。
# 端口优先读 config.yaml 的 server.port，读不到用默认 8080。
$fw = Join-Path $env:WINDIR 'System32\netsh.exe'
$fwPort = '8080'
$cfg = Join-Path $dir 'config.yaml'
if (Test-Path $cfg) {
    $m = Select-String -Path $cfg -Pattern '^\s*port:\s*(\d+)' | Select-Object -First 1
    if ($m) { $fwPort = $m.Matches[0].Groups[1].Value }
}
& $fw advfirewall firewall delete rule name='QinJiaGateway'  | Out-Null
& $fw advfirewall firewall delete rule name='QinJiaGateway_app' | Out-Null
& $fw advfirewall firewall add rule name='QinJiaGateway' dir=in action=allow protocol=TCP localport=$fwPort profile=any | Out-Null
& $fw advfirewall firewall add rule name='QinJiaGateway_app' dir=in action=allow program="$exe" enable=yes profile=any | Out-Null
Write-Host "防火墙规则已配置（程序规则 QinJiaGateway_app + 端口规则 TCP $fwPort）"

if ($r1 -eq 0) {
    Write-Host ''
    Write-Host '=============================================='
    Write-Host ' 系统服务注册成功并已启动（开机自动运行）'
    Write-Host ' 服务名称：QinJiaGateway'
    Write-Host '=============================================='
} else {
    Write-Host "服务注册失败，错误码：$r1"
}
Start-Sleep -Seconds 3
