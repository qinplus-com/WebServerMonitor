# 第三方组件许可声明（THIRD-PARTY NOTICES）

本产品基于以下开源组件构建。各组件版权归其原作者所有，谨致谢意。
所有依赖许可证均允许商业使用与再分发，与本项目的 Apache-2.0 许可证兼容。

## Go 依赖（直接依赖）

| 组件 | 版本 | 许可证 | 用途 |
|---|---|---|---|
| github.com/gin-gonic/gin | v1.10.0 | MIT | HTTP Web 框架 |
| github.com/gin-contrib/cors | v1.7.1 | MIT | CORS 中间件 |
| github.com/golang-jwt/jwt/v5 | v5.2.1 | MIT | JWT 认证 |
| github.com/google/uuid | v1.6.0 | BSD-3-Clause | UUID 生成 |
| github.com/gorilla/websocket | v1.5.3 | BSD-2-Clause | WebSocket（Web 终端） |
| github.com/pkg/sftp | v1.13.6 | BSD-2-Clause | SFTP 文件传输 |
| golang.org/x/crypto | v0.24.0 | BSD-3-Clause | SSH 协议实现 |
| golang.org/x/sys | v0.21.0 | BSD-3-Clause | 系统调用 |
| gopkg.in/yaml.v3 | v3.0.1 | MIT / Apache-2.0 双许可 | 配置解析 |
| gorm.io/gorm | v1.25.10 | MIT | ORM |
| github.com/glebarez/sqlite | v1.11.0 | MIT | GORM SQLite 驱动（纯 Go，无 CGO） |
| modernc.org/sqlite | v1.23.1 | BSD-3-Clause | 纯 Go SQLite 实现（含 libc/mathutil/memory 辅助库，BSD-3-Clause） |

## Go 依赖（主要间接依赖）

| 组件 | 许可证 |
|---|---|
| github.com/bytedance/sonic（及 loader） | Apache-2.0 |
| github.com/cloudwego/base64x、iasm | Apache-2.0 |
| github.com/go-playground/validator/v10（及 locales、universal-translator） | MIT |
| github.com/goccy/go-json | MIT |
| github.com/json-iterator/go | MIT |
| github.com/klauspost/cpuid/v2 | MIT |
| github.com/leodido/go-urn | MIT |
| github.com/mattn/go-isatty | MIT |
| github.com/modern-go/concurrent、reflect2 | Apache-2.0 |
| github.com/pelletier/go-toml/v2 | MIT |
| github.com/phpdave11/gofpdi | MIT（演示版 PDF 组件，随 licgen 工具链） |
| github.com/signintech/gopdf | MIT（授权签发工具链，不随运行时分发） |
| github.com/twitchyliquid64/golang-asm、ugorji/go/codec | Apache-2.0 / MIT |
| golang.org/x/arch、x/net、x/text | BSD-3-Clause |
| google.golang.org/protobuf | BSD-3-Clause |
| github.com/jinzhu/inflection、now、dustin/go-humanize、gabriel-vasile/mimetype、kr/fs、kr/text、pkg/errors、dustin/go-humanize | MIT / BSD 各自仓库声明 |

## 前端依赖

| 组件 | 版本 | 许可证 | 用途 |
|---|---|---|---|
| vue | ^3.4.21 | MIT | 前端框架 |
| vue-router | ^4.3.0 | MIT | 路由 |
| pinia | ^2.1.7 | MIT | 状态管理 |
| element-plus | ^2.7.0 | MIT | UI 组件库 |
| @element-plus/icons-vue | ^2.3.1 | MIT | 图标库 |
| xterm / xterm-addon-fit | ^5.3.0 / ^0.8.0 | MIT | Web 终端 |
| guacamole-common-js | ^1.5.0 | Apache-2.0 | H5 远程桌面（RDP）客户端 |
| axios | ^1.6.8 | MIT | HTTP 客户端 |
| vite / @vitejs/plugin-vue | ^5.2.0 | MIT | 构建工具（仅开发期） |

## 构建与分发工具（不随产品运行时分发）

| 工具 | 许可证 |
|---|---|
| Inno Setup 6 | Inno Setup License（允许免费商用再分发） |
| Go 工具链 | BSD-3-Clause |
| Node.js / npm 生态 | MIT 为主 |

## 独立组件（rdpd 远程桌面引擎）

| 组件 | 许可证 | 说明 |
|---|---|---|
| rdpd | GPL-3.0 | 内置 H5 远程桌面引擎（RDP/VNC 协议栈 → Guacamole 指令流），位于 `rdpd/`，以独立进程经 stdio 与网关通信 |
| grdp（rdpd 内嵌 fork） | GPL-3.0 | 纯 Go RDP 客户端协议库，fork 自 [x90skysn3k/grdp](https://github.com/x90skysn3k/grdp)（上游源自 icodeface/grdp），随源码内嵌于 `rdpd/grdp/` |

## 许可说明

1. 本项目自身代码以 **Apache-2.0** 许可证发布（见 [LICENSE](LICENSE)）。
2. SQLite 相关纯 Go 实现（modernc.org/sqlite 及其 libc 依赖）为 BSD-3-Clause，源自 SQLite 公有领域引擎的 Go 移植，无附加义务。
3. rdpd 及其内嵌的 grdp fork 为独立 GPL-3.0 程序，以独立进程方式与本平台（Apache-2.0）聚合分发——两者进程与代码边界清晰，各自许可互不改变；单独使用 rdpd 时请遵循 GPL-3.0。
4. 如需获取本文件的完整自动化清单，可在仓库根目录执行：
   - Go：`go install github.com/google/go-licenses@latest && go-licenses report ./...`
   - 前端：`npx license-checker --production --json`
