# =============================================================
# H5 远程桌面（Windows RDP）依赖：一键部署 guacamole-server（guacd）
# 用法：右键"使用 PowerShell 运行"，或在 PowerShell 中执行 .\install-guacd.ps1
# 说明：网关默认连接 127.0.0.1:4822（config.yaml 的 guacd_addr）。
#       如需将 guacd 部署到其他机器，修改 guacd_addr 为 "IP:4822"。
# =============================================================
$Port = 4822
$Name = 'guacd-wsm'

Write-Host '== H5 远程桌面依赖部署（guacamole-server / guacd）=='

# 1) 已有 guacd 在运行？
$ok = $false
try {
  $c = New-Object Net.Sockets.TcpClient
  $c.Connect('127.0.0.1', $Port); $c.Close(); $ok = $true
} catch {}
if ($ok) {
  Write-Host "检测到 127.0.0.1:$Port 已有 guacd 在运行，无需重复部署。"
  exit 0
}

# 2) Docker Desktop 方式（推荐）
if (Get-Command docker -ErrorAction SilentlyContinue) {
  docker rm -f $Name 2>$null | Out-Null
  docker run -d --name $Name --restart unless-stopped -p "${Port}:4822" guacamole/guacd
  if ($LASTEXITCODE -eq 0) {
    Write-Host "guacd 已通过 Docker 启动（容器名 $Name）。"
    exit 0
  }
  Write-Host 'Docker 启动失败（请确认 Docker Desktop 已启动）...'
}

Write-Host @'
未检测到可用的 Docker。两种方案任选其一：

A. 安装 Docker Desktop 后重新运行本脚本：
   https://www.docker.com/products/docker-desktop/

B. 在任一 Linux 服务器上运行 guacd（一行命令）：
   docker run -d --name guacd --restart unless-stopped -p 4822:4822 guacamole/guacd
   然后修改网关 config.yaml：
   guacd_addr: "那台服务器IP:4822"
   修改后重启网关服务即可。
'@
exit 1
