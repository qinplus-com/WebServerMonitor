# WebServerMonitor User Guide（勤加软件WEB服务器运维管理平台 · 社区版）

> **Note: Machine Translation.** This guide is machine-translated from the [Chinese original](使用文档.md) and may contain inaccuracies. The Chinese version is the authoritative reference.

> LINUX Server Unified Gateway · One-Stop Ops Management
> Inner Mongolia QinJia Software Co., Ltd.  ·  https://qinplus.com

---

## Table of Contents

1. [Product Overview](#1-product-overview)
2. [Installation & Deployment](#2-installation--deployment)
3. [Configuration](#3-configuration)
4. [First Login](#4-first-login)
5. [Feature Usage](#5-feature-usage)
6. [Management Tool](#6-management-tool)
7. [License Management](#7-version--licensing)
8. [Logs & Backup](#8-logs--backup)
9. [Silent Deployment](#9-silent-deployment)
10. [FAQ](#10-faq)
11. [Technical Support](#11-technical-support)

---

## 1. Product Overview

### 1.1 Positioning

The QinJia Web Server Ops Management Platform is an enterprise-oriented **unified Linux server ops gateway** that integrates a Web SSH terminal, SFTP file management, URL accessibility monitoring, an AI assistant, user permissions, and audit logs into a **single-file Windows service** — ready to use directly in the browser, with zero client installation.

### 1.2 Technical Architecture

| Layer | Technology | Description |
| --- | --- | --- |
| Backend | Go 1.x + Gin | Frontend static assets embedded at compile time via `go:embed` |
| Frontend | Vue 3 + Element Plus | Single-page application with lazy-loaded routes |
| Database | SQLite (pure Go driver) | `CGO_ENABLED=0` static build; no runtime libraries required on target machines |
| Authentication | JWT | 2-hour token + 7-day refresh token |
| Terminal | WebSocket + xterm.js | Real-time bidirectional communication between browser and server |
| Licensing | Community Edition (open-source build) | No license activation flow; no limits on server count or usage duration |

### 1.3 Core Feature Matrix

| # | Feature | Description |
| --- | --- | --- |
| 1 | Web SSH Terminal | Connect to Linux directly from the browser; password/key authentication; select-to-copy, right-click-to-paste |
| 2 | Multi-tab Sessions | Connect to multiple servers simultaneously; parallel operations without interference |
| 3 | SFTP File Management | Browse, upload, download, and edit server files online |
| 4 | URL Accessibility Monitoring | Real-time monitoring of website status, latency, status codes, and SSL certificate validity |
| 5 | AI Assistant | Free AI providing OS-level technical support; can analyze issues together with terminal output |
| 6 | Permissions & Auditing | Multi-role tiered authorization; all server access and operations recorded in audit logs |
| 7 | Online Session Management | View and manage all active terminal connections in real time |
| 8 | Centralized Server Management | Unified maintenance of server information, groups, URLs, and authorizations |
| 9 | Remote Desktop (Windows) | One-click generation of a local `.rdp` file or H5 browser direct connection to the Windows desktop (Guacamole), with no client installation |
| 10 | Demo Mode | Add simulated servers with the `demo:` prefix: 6 simulated hosts, 6 monitoring scenarios, dangerous-command interception — no real server required |

---

## 2. Installation & Deployment

The platform provides two deployment forms: the **Installer Edition** (`.exe`, packaged with Inno Setup) and the **Portable Edition** (`.zip`, no installation required). Both editions are functionally identical and differ only in deployment method.

### 2.1 Installer Edition (recommended for production)

#### 2.1.1 Installation Steps

Double-click `勤加软件WEB服务器运维管理平台-安装版.exe` to launch the installation wizard:

| Step | Description |
| --- | --- |
| ① Welcome | Click "Next" to begin |
| ② License Agreement | Apache-2.0 open-source license; read it and check "I accept the agreement" |
| ③ Version Notes | Shows Community Edition and open-source repository information (skipped automatically when installing over an existing installation) |
| ④ Installation Directory | Defaults to `D:\QinJiaGateway`; customizable |
| ⑤ Software & Service Name | Set the display name (platform title, shortcut name) and the Windows service name (alphanumeric and `_-` only, 1-64 characters, must not start with a digit) |
| ⑥ Data Directory | Business data is stored separately; defaults to `C:\ProgramData\QinJiaGateway`; not removed on uninstall |
| ⑦ Port & Administrator | Access port (default 8080) and initial administrator account/password |
| ⑧ Additional Tasks | Check to create the desktop shortcut and Start Menu group |
| ⑨ Ready to Install | Confirm the configuration and click "Install" |
| ⑩ Installation Complete | Automatically registers the system service, adds firewall rules, generates a random JWT secret, and starts the service |

After installation, two shortcuts appear on the desktop:

- **Platform Browser Access**: uses a dedicated icon; opens the platform directly in the browser
- **Platform Management Tool**: `manager.ps1` (WinForms GUI), for managing service start/stop and backups

> If the Installer Edition detects an existing installation, a dialog pops up asking whether to update; click "Yes" to install directly over it, preserving the old `config.yaml` and business data directory.

#### 2.1.2 Uninstall

Uninstall via "Control Panel - Programs and Features":

- Stops and removes the Windows service
- Removes the installation directory, shortcuts, and Start Menu group
- Removes the firewall inbound rule
- **Keeps** the `C:\ProgramData\QinJiaGateway` business data directory

### 2.2 Portable Edition (offline / portable environments)

#### 2.2.1 File List

Extract `勤加软件WEB服务器运维管理平台-解压版.zip` to any directory (single-level structure):

```
gateway.exe              主程序
manager.ps1              管理工具 GUI
manager.vbs              GUI 启动器（隐藏命令行窗口）
register-service.ps1     注册服务脚本
unregister-service.ps1   卸载服务脚本
启动平台.bat              前台运行
停止平台.bat              停止前台进程
注册系统服务.bat          注册为系统服务（自动 UAC 提权）
卸载系统服务.bat          移除系统服务
LICENSE                  Apache-2.0 开源许可证
qinplus.png              企业 Logo
```

#### 2.2.2 Quick Start

1. Double-click `启动平台.bat`; once the platform starts, the browser opens automatically;
2. Default access URL: `http://localhost:8080/`;
3. Default account: `admin` / `admin123` (**change the password as soon as possible after first login**);
4. Double-click `停止平台.bat` to stop the platform.

#### 2.2.3 Register as a System Service (recommended)

- Right-click `注册系统服务.bat` → **Run as administrator** (the script elevates via UAC automatically);
- Service name `QinJiaGateway`; auto-starts on boot and automatically restarts every 5 seconds after a crash;
- After successful registration, desktop and Start Menu shortcuts are created automatically;
- To remove the service: right-click `卸载系统服务.bat` → Run as administrator (data is preserved).

> The Community Edition (open-source build) has no limits on server count or usage duration; the Installer Edition and the Portable Edition are functionally identical.

### 2.3 Linux Deployment (tar.gz)

Suitable for Linux servers / domestic (Xinchuang) environments (amd64; compiling from source on arm64 architectures such as Kunpeng and Phytium is supported):

```bash
# 1. 解压并赋权
tar -xzf WebServerMonitor-社区版-1.0.0-Linux-x64.tar.gz
cd WebServerMonitor-社区版-1.0.0-Linux-x64
chmod +x gateway *.sh

# 2. 前台试运行（默认端口 8080）
./启动平台.sh

# 3. 生产环境注册 systemd 服务（开机自启 + 崩溃 5 秒自动重启）
sudo ./注册系统服务.sh

# 4. 卸载服务（数据目录保留）
sudo ./卸载系统服务.sh
```

- Default administrator: `admin` / `admin123` (change it immediately on first login);
- Allow the port through the firewall: `firewall-cmd --add-port=8080/tcp --permanent && firewall-cmd --reload`;
- For details, see the bundled 使用说明.txt in the package.

---

## 3. Configuration

### 3.1 Configuration File Locations

| Edition | Configuration File | Data Directory |
| --- | --- | --- |
| Installer Edition | `<安装目录>\config.yaml` | `C:\ProgramData\QinJiaGateway` (customizable) |
| Portable Edition | `<exe 同目录>\config.yaml` | `<exe 同目录>\data` |

> If the configuration file is missing, built-in defaults are used automatically; the environment variables `QJ_PORT` / `QJ_ADMIN_USER` / `QJ_ADMIN_PASS` / `QJ_JWT_SECRET` can also be used to override settings.

### 3.2 config.yaml Field Reference

```yaml
# 服务端配置
server:
  port: 8080              # 浏览器访问端口
  mode: release           # release / debug

database:
  driver: sqlite
  dsn: ./data/gateway.db  # 相对路径基于 exe 所在目录

jwt:
  secret: "随机字符串"     # 安装版自动生成随机密钥
  expire: 7200            # token 过期时间（秒），2 小时
  refresh_expire: 604800  # 刷新 token（秒），7 天

# 默认管理员账号（首次启动自动创建）
admin:
  username: admin
  password: admin123

# 软件品牌信息（安装时可自定义）
app:
  name: 勤加软件WEB服务器运维管理平台
  service_name: QinJiaGateway    # 仅字母数字与 _-

# 版本标识（社区版无授权流程，无需配置授权）
license:
  edition: community      # community | enterprise
```

### 3.3 Port & Firewall

- Changing the port: edit `server.port` in `config.yaml` and restart the service for it to take effect;
- Windows Firewall: the Installer Edition adds the inbound rule automatically; the Portable Edition requires manually allowing the corresponding port;
- Startup fails if the port is already in use; logs are located at `<数据目录>\logs\gateway-YYYYMMDD.log`.

---

## 4. First Login

1. Open `http://<服务器IP>:<端口>/` in a browser (e.g. `http://localhost:8080/`);
2. Enter the administrator account and password (default `admin` / `admin123`);
3. After a successful login you are automatically taken to the Run Overview（运行概览）page;
4. It is **strongly recommended** to change the default password immediately under "User Management".

> The login page uses a dynamic video background plus a glassmorphism login card; the card border has a flowing light-band effect (conic gradient + mask cut-out), and the Edge browser's password-saving feature is supported.

---

## 5. Feature Usage

### 5.1 Run Overview (Dashboard)

The home page after login, centrally displaying:

- Total number of servers and online statistics
- Number of currently active terminal sessions
- Recent logins and command records
- URL accessibility monitoring summary (average latency, normal/abnormal URL counts)
- Visual charts of key metrics

### 5.2 Server Management（服务器管理）

**Path**: Left menu → Server Management

- **List view**: displayed collapsed by group, with a status dot in front of each server (green = online / orange = expiring soon / gray = offline);
- **Add server**: click the "Add Server" button at the top right and fill in the name, group, host IP, port (default 22), account, and authentication method (password / key);
- **Server actions**: each row provides "Connect" (opens the terminal), "Files" (opens SFTP), "Edit", and "Delete" buttons on the right;
- **Group management**: administrators can create/delete groups to categorize servers;
- **URL management**: administrators can associate accessibility-monitored URLs with a server (used by the Dashboard monitoring card);
- **Authorization assignment**: administrators can assign visible servers to regular users to control their access scope.

### 5.3 Web Terminal (SSH)

**Path**: Left menu → Web Terminal

- **Establishing a connection**: click the "+" button on the tab bar and select an authorized server from the dropdown;
- **Multiple tabs**: open several tabs at once to work on the same server or on multiple servers in parallel;
- **Interaction**: select-to-copy and right-click-to-paste (consistent with a native terminal); full xterm key-sequence support;
- **AI integration**: terminal output can be passed to the AI Assistant for analysis;
- **Session persistence**: WebSocket reconnects automatically after a disconnection; long idle periods will not cause a passive disconnection (unless kicked by an administrator).

**Common ops command examples**:

```bash
uname -a          # 内核版本
uptime            # 运行时长与负载
who               # 当前登录用户
df -h             # 磁盘使用情况
free -m           # 内存与交换分区
top / htop        # 进程与资源占用
netstat -tlnp     # 监听端口
iostat -x 1       # 磁盘 IO
```

### 5.4 AI Assistant（AI 智能助手）

**Path**: right side of the Web Terminal page → AI Assistant panel

- **Local LLM**: built-in Ollama with local inference; no internet required;
- **OS-level support**: ask about Linux commands, shell scripts, troubleshooting, system tuning, and more;
- **Works with terminal output**: paste the recent terminal output to the AI for analysis to quickly locate problems;
- **Free to use**: consumes no cloud service quota.

### 5.5 SFTP File Management（文件管理）

**Path**: Left menu → File Management

- **Dual-pane browsing**: local files on the left, remote server files on the right;
- **Upload**: drag local files to the right pane, or click the "Upload" button;
- **Download**: select a remote file and click "Download";
- **Online editing**: double-click a text file to edit and save it online;
- **Permission inspection**: right-click to view file permissions, owner, size, and modification time;
- **Batch transfer**: multi-select files for batch upload/download.

### 5.6 User Management (Admin)

**Path**: Left menu → User Management

- **Account management**: create, edit, and disable user accounts;
- **Roles**: Administrator (full permissions), Regular User (authorized servers only);
- **Permission assignment**: assign visible servers to regular users (they can only connect to / view authorized servers);
- **Password management**: reset user passwords;
- **Login history**: view user login records.

### 5.7 Online Sessions (Admin)

**Path**: Left menu → Online Sessions

- Lists all active SSH connections in real time;
- Shows user, server, connection time, and source IP;
- One-click termination of abnormal sessions (prevents account misuse or long-term occupation);
- Linked with the audit logs, so all commands within a session can be traced.

### 5.8 Audit Logs (Admin)

**Path**: Left menu → Audit Logs

| Log Type | Content Recorded |
| --- | --- |
| Login logs | Account, IP, time, login result (success/failure) |
| Command logs | All commands executed in the terminal and output summaries |
| SFTP logs | File upload/download records, operator, file name |

- Filter by time, user, and type;
- Export to Excel for archiving;
- All operations are fully traceable, meeting compliance audit requirements.

### 5.9 URL Accessibility Monitoring（网址可访问性监控）

**Path**: Run Overview page → URL Monitoring card

- All monitored URLs are checked automatically in the background every 60 seconds;
- Each card shows: status dot, domain, HTTP status code, latency, and remaining days of SSL certificate validity;
- Status color rules:
  - Green: HTTP 200 and certificate > 30 days
  - Yellow: HTTP 200 but certificate ≤ 30 days
  - Red: 4xx / 5xx / connection failure
  - Red-yellow-blue flashing: certificate expired or service failure + certificate ≤ 30 days
- Abnormal cards are automatically pinned to the top;
- DingTalk alerts supported (custom group robot / enterprise application robot).

### 5.10 About the Platform

**Path**: Left menu → About

- Platform name and logo;
- 8 feature cards with flowing-light animation (a conic-gradient comet loops along the card border);
- Company introduction and contact information;
- Copyright notice (encrypted storage + integrity verification to prevent tampering).

### 5.11 Remote Desktop (Windows Servers)

**Path**: Server Management → a Windows-type server row → "Remote Desktop"

When adding a server, select Windows as the "Server Type" (the default port automatically switches to 3389) to use Remote Desktop with Windows servers. Two ways to open it:

| Method | Description |
| --- | --- |
| Local RDP | Generates a standard `.rdp` file for download; double-click to launch the local mstsc connection (no server-side component required) |
| H5 | Operate the Windows desktop directly in the browser: bidirectional clipboard sync, adaptive window scaling, fullscreen; no client installation |
| Copy Address | Copies `IP:端口` for manual connection via local mstsc |

The H5 method requires deploying [guacamole-server](https://guacamole.apache.org/) (guacd) first:

```bash
docker run --name guacd -d -p 4822:4822 guacamole/guacd
```

and configuring `guacd_addr` in the gateway's `config.yaml` (default `127.0.0.1:4822`; set it to `-` to disable the H5 entry; the local RDP method is unaffected).

> Sessions are established using a 60-second one-time token (the token is discarded after use); server passwords are never cached in the browser; Windows servers without a configured password are blocked with a prompt.

---

## 6. Management Tool

The desktop shortcut "Platform Management Tool" opens `manager.ps1` (native WinForms GUI; double-clicking auto-elevates via UAC).

### 6.1 Feature List

| Feature | Description |
| --- | --- |
| Service start / stop / restart | One-click operation of the Windows service; button state is automatically linked with the service status |
| Real-time status | Three-color indicator: green (running) / orange (stopped) / gray (not registered) |
| Back Up Now | Automatically stops the service → backs up the database → automatically restarts the service |
| Restore Selected Backup | Restores after verifying file integrity to avoid data corruption |
| Open Platform Web Page | Opens the platform in the default browser |
| Open Data Directory | Jumps to the `data_dir` configured in `config.yaml` |
| Open Configuration File | Opens `config.yaml` in Notepad |
| License Status | Shows the current edition (Community Edition) and server count |

### 6.2 Backup Directory

- Default backup directory: `C:\ProgramData\QinJiaGateway\backups`;
- The backup directory can be customized in the Management Tool (**rejects** the installation directory, the platform data directory, and empty paths; non-existent directories are not created automatically);
- The backup directory setting is persisted in `C:\ProgramData\QinJiaGateway\manager.conf`;
- Uninstalling and reinstalling the platform does not affect backup files.

> When the service fails to start, the popup shows the failure reason and the log path (logs are located under `logs\` in the data directory).

---

## 7. Version & Licensing

This platform is distributed as the **Community Edition** (open source under the Apache License 2.0):

- No license activation flow: no activation required, no server count limit, no usage duration limit;
- The "Version Notes" page in the installer wizard shows edition and open-source repository information;
- The Management Tool's "License Status" and the platform's "Settings" page display the `社区版` (Community Edition) label;
- Open-source repositories: `github.com/qinplus-com/WebServerMonitor` / `gitee.com/qinpluscom/web-server-monitor`.

For commercial licensing (Enterprise Edition / brand OEM customization / Xinchuang adaptation services), please contact the vendor (see the "Technical Support" chapter).

---

## 8. Logs & Backup

### 8.1 Logs

- Path: `<数据目录>\logs\gateway-YYYYMMDD.log` (generated daily);
- Content: complete logs including GIN requests, service start/stop, terminal commands, SFTP operations, etc.;
- To troubleshoot startup failures: check the end of the current day's log file.

### 8.2 Backup

- **Manual backup**: Management Tool → Back Up Now;
- **Restore**: Management Tool → select a backup file → Restore;
- The service is stopped automatically during backup and restarted automatically upon completion, ensuring data consistency;
- Backup sets are stored together in the backup directory, which is customizable.

---

## 9. Silent Deployment

The Installer Edition supports silent deployment (unattended installation), suitable for bulk distribution:

```bat
setup.exe /VERYSILENT ^
  /APPNAME=顺和运维管理平台 ^
  /SVC=ShunHeOps ^
  /PORT=18100 ^
  /ADMIN=admin ^
  /PASS=YourStrongPassword ^
  /DIR="D:\ShunHeOps"
```

| Parameter | Description |
| --- | --- |
| `/VERYSILENT` | Silent deployment, no UI |
| `/APPNAME=` | Software display name (platform title, shortcut name) |
| `/SVC=` | Windows service name (1-64 characters, alphanumeric plus `_-`, must not start with a digit) |
| `/PORT=` | Access port |
| `/ADMIN=` | Administrator username |
| `/PASS=` | Administrator password |
| `/DIR=` | Installation directory |

> `/VERYSILENT` mode skips the installation-complete prompt window for fully unattended deployment; in non-silent mode, a multi-select checkbox page appears when installation completes (with "Open Web Platform" and "Open Service Management Tool" checked by default).

---

## 10. FAQ

### Q1: Forgot the administrator password?

After stopping the service, run the reset command in the program directory (the following example resets `admin` to a new password):

```bat
gateway.exe -set-admin-user admin -set-admin-pass 新密码 -config config.yaml -data "数据目录"
```

After that, start the service and log in with the new password.

### Q2: Port already in use?

Change `server.port` in `config.yaml`, or override it via the `QJ_PORT` environment variable; also allow the new port through the Windows Firewall.

### Q3: Service fails to start (1053)?

Check the current day's log under `logs\` in the data directory to locate the cause (common causes: port already in use, insufficient data directory permissions).

### Q4: Where is the data directory?

| Edition | Data Directory | Log Directory |
| --- | --- | --- |
| Installer Edition | `C:\ProgramData\QinJiaGateway` | `<数据目录>\logs\` |
| Portable Edition | `<exe 同目录>\data` | `<数据目录>\logs\` |

### Q5: How to migrate the platform?

1. Stop the service;
2. Copy the entire data directory to the new machine;
3. Reinstall / re-extract the platform program;
4. Edit `config.yaml` to point to the new data directory (or keep the default);
5. Start the service.

### Q6: Blank page in the browser?

- Confirm the service is running (the Management Tool status is green);
- Confirm the port is not blocked by the firewall;
- Clear the browser cache or hard-refresh with `Ctrl + F5`;
- Check the logs for startup errors.

### Q7: Terminal cannot connect to a server?

- Confirm the server's SSH service is running (`systemctl status sshd`);
- Confirm the server IP, port, and account/password are correct;
- Confirm the server firewall allows port 22;
- Confirm network reachability from the gateway server to the target server (`ping` / `telnet`).

---

## 11. Technical Support

- **Company Name**: Inner Mongolia QinJia Software Co., Ltd.
- **Official Website**: https://qinplus.com
- **Phone**: 18047115759
- **Licensing & Customization**: the Community Edition is free to use; for Enterprise Edition / brand OEM / Xinchuang adaptation and other commercial licensing, please contact the vendor

### 11.1 Log Collection

When you encounter a problem, please provide the following information so it can be located quickly:

1. Platform version (see the About page);
2. The current day's log file `<数据目录>\logs\gateway-YYYYMMDD.log`;
3. Reproduction steps and screenshots;
4. Browser console errors (F12 developer tools).

### 11.2 Version Notes

- Installer Edition: compiled with Inno Setup; supports silent deployment and brand customization
- Portable Edition: ready to use from a single-level directory; includes the `manager.ps1` management tool
- Frontend static assets are compiled into the exe via `go:embed`, enabling single-file execution
- Pure Go SQLite driver with `CGO_ENABLED=0` static compilation; the target machine requires no runtime libraries

---

*This guide was last updated in October 2026.*
