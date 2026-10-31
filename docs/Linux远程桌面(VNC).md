# Linux 远程桌面（VNC）部署文档 —— 国产操作系统适配指南

> 适用产品：勤加软件 WEB 服务器运维管理平台（H5 远程桌面 · VNC 模式）
> 适用对象：国产及通用 Linux 服务器 / 桌面操作系统（银河麒麟、统信 UOS、openEuler、中标麒麟、深度、openKylin、红旗 等）
> 更新日期：2026-10-31（银河麒麟高级服务器 V11 已实测通过）

---

## 1. 平台工作原理与约定

平台通过 **RFB/VNC 协议** 连接目标机的图形桌面，约定如下：

| 项目 | 约定 | 说明 |
|---|---|---|
| VNC 端口 | `5901`（display `:1`） | 服务器编辑中"VNC 端口"填 5901；`:2` 对应 5902，以此类推 |
| VNC 密码 | **与平台服务器编辑中填写的登录密码一致** | VNC 认证与 SSH 认证共用同一密码字段 |
| 监听范围 | 所有网卡（不能仅 localhost） | 网关主机跨机连接目标机 |
| 桌面尺寸 | 建议 `1920x1080`、深度 24 | 平台工具栏可随时切换分辨率（SSH 执行 xrandr） |
| 分辨率切换 | 平台经 SSH 执行 `xrandr` | 要求 SSH 账号对该 X 显示有授权（见第 8.4 节） |
| 连接链路 | 浏览器 → 网关(rdpd) → 目标机:5901 | 仅需目标机放行 5901，浏览器无需直连 |

> 连接失败时平台会给出排查提示；本文第 9 节为完整排障手册。

---

## 2. 通用部署流程（总览）

无论哪种发行版，部署都由 5 步组成：

```
① 安装 VNC 服务端（TigerVNC）
② 安装/确认桌面环境（UKUI / DDE / MATE / GNOME / XFCE …）
③ 设置 VNC 密码（= 平台登录密码）
④ 配置启动方式（会话配置 / xstartup）并启动 :1 桌面
⑤ 放行防火墙 5901 + 配置开机自启 → 平台侧填 VNC 端口并验收
```

按包管理家族分两条主线：

- **RPM 系**（麒麟服务器版、openEuler、中标麒麟、Anolis/OpenCloudOS/TencentOS 等）：`yum/dnf + tigervnc-server + systemd vncserver@ 模板单元`
- **DEB 系**（统信 UOS、深度、openKylin、红旗桌面版 等）：`apt + tigervnc-standalone-server + 自制 systemd 单元`

---

## 3. RPM 系系统通用部署（麒麟服务器 / openEuler / 中标麒麟 等）

### 3.1 安装软件包

```bash
# VNC 服务端（RHEL/CentOS/openEuler/麒麟系包名一致）
yum install -y tigervnc-server        # openEuler/麒麟 V10+ 用 dnf 亦可：dnf install -y tigervnc-server
```

若系统尚未安装桌面环境，按各系统章节（第 5 节）先装桌面。

### 3.2 设置 VNC 密码

```bash
vncpasswd
# 交互输入两次密码（建议与平台登录密码一致，如 root 的密码）
# 也可一行完成（推荐，便于与平台密码保持一致）：
# echo '你的登录密码' | vncpasswd -f > ~/.vnc/passwd && chmod 600 ~/.vnc/passwd
```

### 3.3 会话配置（TigerVNC ≥ 1.9 推荐方式）

```bash
# /etc/tigervnc/vncserver.users   —— display 与用户的映射
:1=root

# /etc/tigervnc/vncserver-config-defaults   —— 全局默认参数
geometry=1920x1080
depth=24
localhost=no
session=ukui          # 按实际桌面填写：ukui / mate / gnome / dde …
alwaysshared          # 允许网关等多个查看端同时连接
```

`session=` 的取值 = `/usr/share/xsessions/` 下 `.desktop` 文件名（去后缀），用 `ls /usr/share/xsessions/` 查询。

> 注意：systemd 方式启动由 `vncsession` 管理，**不读取 `~/.vnc/xstartup`**，必须用 `session=` 指定会话；仅手工运行 `vncserver :1` 时才使用 xstartup（见 3.5）。

### 3.4 启动与开机自启

```bash
systemctl enable --now vncserver@:1.service
ss -tlnp | grep 5901          # 确认监听
tail -20 ~/.vnc/*.log         # 查看会话日志
```

### 3.5 手工启动方式（可选，兼容老版本 CentOS 6/7）

```bash
cat > ~/.vnc/xstartup <<'EOF'
#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
exec dbus-launch --exit-with-session ukui-session    # 按实际桌面替换
EOF
chmod +x ~/.vnc/xstartup
vncserver :1 -geometry 1920x1080 -depth 24
```

老版本（如 CentOS 7 的 TigerVNC 1.8）也可继续使用传统 `vncserver@:1.service`+`xstartup` 模板方式。

---

## 4. DEB 系系统通用部署（统信 UOS / 深度 / openKylin / 红旗桌面 等）

### 4.1 安装软件包

```bash
sudo apt update
sudo apt install -y tigervnc-standalone-server tigervnc-common dbus-x11
```

### 4.2 设置 VNC 密码与 xstartup

```bash
echo '你的登录密码' | vncpasswd -f > ~/.vnc/passwd && chmod 600 ~/.vnc/passwd

mkdir -p ~/.vnc
cat > ~/.vnc/xstartup <<'EOF'
#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
exec dbus-launch --exit-with-session ukui-session   # DDE 桌面改为 startdde，MATE 改为 mate-session
EOF
chmod +x ~/.vnc/xstartup
```

### 4.3 启动

```bash
vncserver :1 -geometry 1920x1080 -depth 24 -localhost no -alwaysshared
ss -tlnp | grep 5901
```

### 4.4 自制 systemd 开机自启（DEB 系无现成模板单元）

```bash
sudo tee /etc/systemd/system/vncserver-my.service > /dev/null <<'EOF'
[Unit]
Description=VNC Server (:1)
After=network.target

[Service]
Type=forking
User=你的用户名
ExecStart=/usr/bin/vncserver :1 -geometry 1920x1080 -depth 24 -localhost no -alwaysshared
ExecStop=/usr/bin/vncserver -kill :1
Restart=on-failure

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable --now vncserver-my.service
```

---

## 5. 各国产操作系统适配明细

### 5.1 银河麒麟高级服务器操作系统 V10 / V11（RPM 系 · 已实测 ✅）

**环境实测**：Kylin Linux Advanced Server V11 (Swan25) + TigerVNC 1.13.1 + UKUI 4.10

```bash
# ① 安装（最小化安装通常只有 Xvnc，缺启动器，必须补装 tigervnc-server）
dnf install -y tigervnc-server

# ② VNC 密码（与平台登录密码一致）
echo '登录密码' | vncpasswd -f > ~/.vnc/passwd && chmod 600 ~/.vnc/passwd

# ③ 会话配置
printf ':1=root\n' > /etc/tigervnc/vncserver.users
printf 'geometry=1920x1080\ndepth=24\nlocalhost=no\nsession=ukui\n' > /etc/tigervnc/vncserver-config-defaults

# ④ 启动 + 开机自启
systemctl enable --now vncserver@:1.service

# ⑤ 防火墙
firewall-cmd --add-port=5901/tcp --permanent && firewall-cmd --reload
```

**⚠️ 麒麟 UKUI 单会话限制（重要）**：UKUI 硬编码"同一用户同时仅允许一个图形会话"。若 root 已在物理控制台登录桌面，VNC 再起 root 会话会弹出"当前用户已经在系统中登录"，约 60 秒后 VNC 会话自动退出（`logout-timeout` 默认 60）。**推荐专用 VNC 用户方案**（与控制台会话互不干扰）：

```bash
# 1) 建专用用户（密码与平台服务器编辑中的密码保持一致，VNC 认证用）
useradd -m -s /bin/bash vncuser
echo 'vncuser:平台登录密码' | chpasswd

# 2) 该用户的 VNC 密码
mkdir -p /home/vncuser/.vnc
echo '平台登录密码' | vncpasswd -f > /home/vncuser/.vnc/passwd
chown -R vncuser:vncuser /home/vncuser/.vnc && chmod 600 /home/vncuser/.vnc/passwd

# 3) :1 改由 vncuser 提供
printf ':1=vncuser\n' > /etc/tigervnc/vncserver.users

# 4) 允许 root 经 SSH 对该 X 显示执行 xrandr（平台分辨率切换依赖）
tee /etc/X11/xinit/xinitrc.d/99-root-xhost.sh > /dev/null <<'EOF'
#!/bin/sh
[ -x /usr/bin/xhost ] && xhost +si:localuser:root >/dev/null 2>&1
EOF
chmod +x /etc/X11/xinit/xinitrc.d/99-root-xhost.sh

# 5) 重启服务
systemctl restart vncserver@:1.service
```

> 若服务器无控制台占用（纯虚拟机/无头机房），直接用 root 跑 :1 也可以；一旦有人再从控制台登录 root 桌面，二者互挤，故生产环境建议 vncuser 方案。

### 5.2 银河麒麟桌面操作系统 V10（RPM/DEB 混合，桌面版预装 UKUI）

桌面版自带 UKUI 且控制台常有登录用户，**推荐第 3 节标准流程 + 专用 VNC 用户**（同 5.1 第②~⑤步，把用户换成桌面版普通用户）。桌面版基于 Ubuntu 系的版本走 DEB 系（第 4 节）流程，xstartup 中 `exec dbus-launch --exit-with-session ukui-session`。

**替代方案（镜像物理屏幕）**：若希望 VNC 看到的就是物理控制台当前画面（非独立会话，无单会话冲突），用 x11vnc：

```bash
apt install -y x11vnc        # 或 yum install -y x11vnc
x11vnc -rfbport 5901 -passwd '登录密码' -display :0 -forever -shared -loop
# 建议封装成 systemd 服务开机自启
```

### 5.3 统信 UOS 服务器版 V20（DEB 系）

UOS 服务器版默认无图形界面，仓库内提供 UKUI 桌面组件：

```bash
# ① VNC
apt update && apt install -y tigervnc-standalone-server tigervnc-common dbus-x11

# ② 桌面环境（未预装时；包名以 `apt search ukui` 实际结果为准）
apt install -y ukui-session ukui-panel ukui-desktopmanager peony-qt

# ③ 密码 + xstartup（见 4.2，exec ukui-session）→ ④ 启动（见 4.3）→ ⑤ 自制 systemd（见 4.4）
# ⑥ 防火墙（UOS Server 多用 firewalld）：firewall-cmd --add-port=5901/tcp --permanent && firewall-cmd --reload
```

### 5.4 统信 UOS 桌面专业版 / 家庭版（DEB 系 · DDE 桌面）

预装 DDE（Deepin 桌面），走第 4 节标准流程，仅 xstartup 会话改为 DDE：

```bash
exec dbus-launch --exit-with-session startdde
```

注意事项：
- UOS 桌面版默认安全策略较严，若 VNC 被拒绝，检查 `sudo ufw status` 并放行 `sudo ufw allow 5901/tcp`
- 与 5.1 同理，DDE 也可能受"同用户重复登录"限制，建议专用 VNC 用户运行独立会话
- 家庭版部分型号为 ARM（飞腾/鲲鹏），命令一致

### 5.5 openEuler 22.03 / 24.03（RPM 系）

```bash
dnf install -y tigervnc-server
# 桌面环境二选一（未预装时）：
dnf groupinstall "UKUI"        # 开放麒麟原生桌面（推荐）
dnf groupinstall "GNOME"       # 或 GNOME：dnf groupinstall "Server with GUI"
# 其余同第 3 节：vncpasswd → /etc/tigervnc/ 两份配置（session=ukui 或 gnome）
# → systemctl enable --now vncserver@:1 → firewall-cmd 放行 5901
```

### 5.6 中标麒麟（NeoKylin）V7 / V10（RPM 系 · MATE 桌面）

```bash
yum install -y tigervnc-server
# MATE 桌面未预装时：yum install -y mate-session-manager mate-terminal caja
# 传统方式（老版 TigerVNC）：
cat > ~/.vnc/xstartup <<'EOF'
#!/bin/sh
unset SESSION_MANAGER
unset DBUS_SESSION_BUS_ADDRESS
exec dbus-launch --exit-with-session mate-session
EOF
chmod +x ~/.vnc/xstartup
vncpasswd && vncserver :1 -geometry 1920x1080 -depth 24
# 开机自启：systemctl enable vncserver@:1（V7 需先编辑 /etc/sysconfig/vncservers：
#   VNCSERVERS="1:root"  VNCSERVERARGS[1]="-geometry 1920x1080 -depth 24"）
```

### 5.7 深度 deepin 20 / 23（DEB 系 · DDE）

与 5.4 相同：`tigervnc-standalone-server` + xstartup `exec dbus-launch --exit-with-session startdde`。deepin 23 仓库较新，若无 tigervnc 包可改用 `tigervnc-xorg-extension` 来源或 x11vnc 镜像方案。

### 5.8 openKylin（DEB 系 · UKUI）

```bash
sudo apt install -y tigervnc-standalone-server dbus-x11
# 密码/xstartup 见 4.2（exec ukui-session），启动见 4.3，systemd 见 4.4
```

### 5.9 红旗 Linux（桌面版/服务器版）

- 桌面版（基于 Debian 系、DDE 或 UKUI）：按第 4 节 DEB 流程，会话按实际桌面选 `startdde` 或 `ukui-session`
- 服务器版（RPM 系）：按第 3 节 RPM 流程

### 5.10 其他 RPM 系国产服务器（龙蜥 Anolis / OpenCloudOS / TencentOS Server）

无预装桌面，通常无需远程桌面；确需时：`dnf install -y tigervnc-server` + GNOME（`dnf groupinstall "Server with GUI"`），其余同第 3 节。

---

## 6. 快速对照表

| 操作系统 | 包管理 | VNC 包 | 桌面会话 | 启动方式 | 关键风险 |
|---|---|---|---|---|---|
| 银河麒麟服务器 V10/V11 | dnf/yum | tigervnc-server | `session=ukui` | systemd vncserver@ | UKUI 单会话 → 用 vncuser |
| 银河麒麟桌面 V10 | apt/dnf 视版本 | tigervnc-standalone-server / server | `ukui` / xstartup | systemd / vncserver | 单会话；或用 x11vnc 镜像 |
| 统信 UOS 服务器 | apt | tigervnc-standalone-server | `ukui-session` | vncserver + 自制 unit | 需另装桌面组件 |
| 统信 UOS 桌面 | apt | tigervnc-standalone-server | `startdde` | vncserver + 自制 unit | 单会话；ufw 放行 |
| openEuler | dnf | tigervnc-server | `ukui`/`gnome` | systemd vncserver@ | — |
| 中标麒麟 | yum | tigervnc-server | `mate-session` | vncservers/xstartup | 老版本用 sysconfig 配置 |
| 深度 deepin | apt | tigervnc-standalone-server | `startdde` | vncserver + 自制 unit | 仓库版本差异 |
| openKylin | apt | tigervnc-standalone-server | `ukui-session` | vncserver + 自制 unit | — |
| 红旗 Linux | apt/dnf 视版本 | 同上族 | `startdde`/`ukui-session` | 对应族流程 | 版本差异大 |
| Anolis/OpenCloudOS/TencentOS | dnf/yum | tigervnc-server | `gnome` | systemd vncserver@ | 需另装桌面组 |

---

## 7. 防火墙放行汇总

```bash
# firewalld（麒麟/openEuler/中标麒麟/Anolis 等）
firewall-cmd --add-port=5901/tcp --permanent && firewall-cmd --reload

# ufw（UOS/deepin/openKylin/红旗桌面等）
sudo ufw allow 5901/tcp

# iptables（无上述二者的老系统）
iptables -I INPUT -p tcp --dport 5901 -j ACCEPT
```

云服务器另需在厂商控制台安全组放行 5901。

---

## 8. 常见问题排查（按现象）

### 8.1 平台提示"无法连接 VNC 服务 … i/o timeout"
目标机 5901 未放行（防火墙/安全组）或 VNC 只监听 127.0.0.1（配置缺 `localhost=no`）。按第 7 节放行；`ss -tlnp | grep 5901` 确认监听 `0.0.0.0`。

### 8.2 平台提示"目标端口不是 VNC/RFB 服务"
端口填错（如把 SSH 22 填到 VNC 端口），或 5901 上跑的是别的服务。

### 8.3 平台提示"VNC 认证失败"
平台密码与 `vncpasswd` 不一致。重设：`echo '平台密码' | vncpasswd -f > ~/.vnc/passwd && chmod 600 ~/.vnc/passwd` 后重启 vncserver@:1。

### 8.4 桌面能连上，但平台切换分辨率报错 / 不生效
平台经 **SSH 账号** 执行 `export DISPLAY=:1; xrandr`：
- SSH 用户与 VNC 会话用户**相同**：天然有授权
- **不同**（如 5.1 的 vncuser 方案）：需第 5.1 步骤 4) 的 xhost 钩子，且该钩子在"会话启动时"生效——改完后重启 vncserver@:1 使新会话带上授权
- 报"未获取到可用分辨率"：目标机无桌面/xrandr 缺失，`yum install -y xorg-x11-apps`（DEB 系 `apt install -y x11-xserver-utils`）

### 8.5 VNC 连上后黑屏 / 一会儿就断
- 麒麟 UKUI"当前用户已经在系统中登录"单会话冲突 → 见 5.1 vncuser 方案
- 会话日志 `~/.vnc/*.log` 中 GL/MESA 报错属虚拟显卡软件渲染，一般不影响使用
- 桌面环境未装全：`ls /usr/share/xsessions/` 确认有会话项，按第 5 节补装

### 8.6 服务重启后 5901 没了
未配置开机自启：`systemctl enable --now vncserver@:1.service`（DEB 系见 4.4）。

---

## 9. 平台侧配置与验收清单

1. 平台"服务器管理 → 编辑"中：操作系统选 Linux、**VNC 端口填 5901**、密码填目标机登录密码（= VNC 密码）
2. 列表点击"远程桌面（H5 · VNC）"，预期 30 秒内出现目标机桌面，工具栏状态"已连接"
3. 验收项：
   - [ ] 画面正常、鼠标键盘可操作（中文输入正常）
   - [ ] 分辨率下拉含 1280x720/1600x900/1920x1080/2560x1440 等 16:9 档，切换后画面自动适配
   - [ ] 断开后重连正常；目标机重启后 VNC 自启（`systemctl is-enabled vncserver@:1`）

> 平台侧无需在目标机安装任何代理/客户端软件，仅依赖：TigerVNC 服务端 + 桌面环境 + SSH（root 或具备 sudo 的账号）。
