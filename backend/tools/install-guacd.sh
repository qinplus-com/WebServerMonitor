#!/usr/bin/env bash
# =============================================================
# H5 远程桌面（Windows RDP）依赖：一键部署 guacamole-server（guacd）
# 用法：bash install-guacd.sh
# 说明：网关默认连接 127.0.0.1:4822（config.yaml 的 guacd_addr）。
#       如需将 guacd 部署到其他机器，修改 guacd_addr 为 "IP:4822"。
# =============================================================
PORT=4822
NAME=guacd-wsm

echo "== H5 远程桌面依赖部署（guacamole-server / guacd）=="

# 0) root 检查（apt/yum/docker 均需要）
if [ "$(id -u)" != "0" ] && command -v sudo >/dev/null 2>&1; then
  SUDO=sudo
else
  SUDO=
fi

# 1) 已有 guacd 在运行？
if ($SUDO bash -c "exec 3<>/dev/tcp/127.0.0.1/$PORT") 2>/dev/null; then
  echo "检测到 127.0.0.1:$PORT 已有 guacd 在运行，无需重复部署。"
  exit 0
fi

# 2) Docker 方式（推荐）
if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  docker rm -f $NAME >/dev/null 2>&1 || true
  # 仅绑定本机回环，避免 guacd 暴露到公网（gateway 与 guacd 同机）
  if docker run -d --name $NAME --restart unless-stopped -p 127.0.0.1:$PORT:4822 guacamole/guacd; then
    echo "guacd 已通过 Docker 启动（容器名 $NAME）。"
    exit 0
  fi
  echo "Docker 启动失败（镜像拉取可能受限），尝试其他方式..."
fi

# 3) Debian / Ubuntu：apt 安装
if command -v apt-get >/dev/null 2>&1; then
  echo "尝试通过 apt 安装 guacamole-server ..."
  if $SUDO apt-get update && $SUDO apt-get install -y guacamole-server; then
    $SUDO systemctl enable --now guacd 2>/dev/null || $SUDO guacd -b 127.0.0.1 -l $PORT -f &
    echo "guacd 已安装并启动。"
    exit 0
  fi
fi

# 4) CentOS / RHEL：yum + EPEL
if command -v yum >/dev/null 2>&1; then
  echo "尝试通过 yum/EPEL 安装 guacamole-server ..."
  if $SUDO yum install -y epel-release && $SUDO yum install -y guacamole-server; then
    $SUDO systemctl enable --now guacd
    echo "guacd 已安装并启动。"
    exit 0
  fi
fi

echo "自动部署失败。请任选其一："
echo "  A. 先安装 Docker 后重跑本脚本：curl -fsSL https://get.docker.com | bash"
echo "  B. 在其他 Linux 服务器上运行：docker run -d --name guacd --restart unless-stopped -p 4822:4822 guacamole/guacd"
echo "     然后修改网关 config.yaml：guacd_addr: \"那台服务器IP:4822\""
exit 1
