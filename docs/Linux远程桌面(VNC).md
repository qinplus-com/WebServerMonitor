# Linux 远程桌面（VNC）部署指南

网关对 Linux 服务器提供 H5 浏览器远程桌面，底层通过 VNC（RFB）协议连接目标机的桌面会话。以 CentOS 为例（CentOS 7 / Stream 8 / Stream 9 均适用），其他 RHEL 系发行版步骤相同。

> 前提：目标机已安装桌面环境（GNOME/Xfce 等）。最小安装的服务器需先 `yum groupinstall "Server with GUI"`。

## 一、目标机配置（CentOS）

### 1. 安装 tigervnc

```bash
yum install -y tigervnc-server
```

### 2. 设置 VNC 访问密码

```bash
vncpasswd
```

按提示输入密码（view-only 密码可留空）。

> 重要：平台侧使用「服务器编辑中填写的登录密码」尝试 VNC 认证。请将 vncpasswd 密码设置为与登录密码一致；或将 vncpasswd 置空（跳过设置、直接回车），目标机将允许免密连接（仅建议内网使用）。

### 3. 启动 VNC 服务（:1 桌面 → 端口 5901）

CentOS 7：

```bash
# 复制服务模板（将 <user> 替换为实际登录用户名）
cp /lib/systemd/system/vncserver@.service /etc/systemd/system/vncserver@:1.service
vi /etc/systemd/system/vncserver@:1.service
# 将 <USER> 全部替换为登录用户名（ExecStart/ExecStop/PIDFile 三处）

systemctl daemon-reload
systemctl enable --now vncserver@:1.service
```

CentOS Stream 8/9（推荐直接用用户会话方式）：

```bash
# 以登录用户执行，:1 即端口 5901
vncserver :1

# 开机自启（用户级 systemd 实例）
loginctl enable-linger <user>
systemctl --user enable vncserver@:1.service
```

### 4. 防火墙放行

```bash
firewall-cmd --permanent --add-port=5901/tcp
firewall-cmd --reload
```

### 5. 验证

```bash
ss -tlnp | grep 5901    # 确认 VNC 服务已监听
```

## 二、平台侧配置

1. 网关「服务器管理」→ 编辑目标 Linux 服务器 → 打开「远程桌面」开关，VNC 端口填 `5901`（对应 :1；:2 则填 5902，以此类推）。
2. 保存后服务器列表出现「远程桌面」→「H5 浏览器桌面（VNC）」入口，点击即可在浏览器中操作 Linux 桌面。

## 三、常见问题

| 现象 | 处理 |
| --- | --- |
| 提示「无法连接 VNC 服务」 | 目标机 VNC 未启动或防火墙未放行，按上文 1~4 步检查 |
| 提示「VNC 认证失败」 | 目标机 vncpasswd 密码与平台服务器密码不一致：重设 vncpasswd 与登录密码一致，或清空 vncpasswd 免密 |
| 提示「目标端口不是 VNC/RFB 服务」 | VNC 端口填错（确认 `ss -tlnp | grep 590` 实际监听端口） |
| 连接后黑屏 | GNOME 会话已被本地占用时改用独立会话（tigervnc 默认独立 Xvnc 桌面即无此问题）；检查 `/root/.vnc/xstartup` 或用户级服务日志 |
| 画面卡顿 | 局域网内为无损 PNG 增量推送；跨公网建议降低目标机桌面分辨率或关闭桌面特效 |
