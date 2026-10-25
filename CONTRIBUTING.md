# 参与贡献（CONTRIBUTING）

感谢关注并参与 勤加软件WEB服务器运维管理平台（QinJia Gateway）！
无论是报告缺陷、提交功能建议还是贡献代码，都欢迎。

## 行为准则

保持友善与专业；讨论聚焦技术本身；尊重不同使用场景与观点。

## 如何报告缺陷（Bug Report）

提交 Issue 时请尽量包含：

1. **环境信息**：操作系统 / 浏览器 / 平台版本（安装版、解压版或 Linux 包）
2. **复现步骤**：可直接照做的最小复现路径
3. **期望行为 vs 实际行为**
4. **日志**：数据目录 `logs/gateway-YYYYMMDD.log` 中相关时间段的片段（注意脱敏：勿贴服务器 IP、账号密码、证书私钥）
5. 若与演示模式相关：注明使用的演示账号场景（demo / demo123）

> 安全漏洞请**不要**公开 Issue，请通过仓库的私下安全报告渠道（Security Advisories 或联系维护者）提交，我们会在确认后于下一个补丁版本致谢。

## 如何提交功能建议（Feature Request）

- 先检索现有 Issue，避免重复
- 说明**场景与痛点**（解决什么问题），比直接给实现方案更受欢迎
- 重大特性（新协议、新探针类型、架构调整）建议先提 Issue 讨论，达成一致后再编码

## 开发环境

| 依赖 | 版本 |
|---|---|
| Go | ≥ 1.21 |
| Node.js | ≥ 18 |
| Inno Setup 6（仅打 Windows 安装包需要） | 6.x |

```bash
# 后端本地运行（默认 8080 端口，数据库自动创建于 exe 同目录）
cd backend && go run . 

# 前端开发（Vite 热更新，代理至后端）
cd frontend && npm install && npm run dev

# 完整构建
cd frontend && npm run build   # 产物进入 backend/static
cd ../backend && CGO_ENABLED=0 go build -o gateway.exe .
```

> 注意：`frontend` 构建产物输出到 `backend/static`，后端通过 `go:embed` 嵌入——改前端后需重新编译后端二进制。

## 提交代码（Pull Request）

1. 从 `main` 拉出特性分支：`git checkout -b feat/your-feature`
2. 提交信息遵循 Conventional Commits：
   - `feat:` 新功能　`fix:` 缺陷修复　`docs:` 文档　`refactor:` 重构　`test:` 测试　`chore:` 构建/工具
   - 示例：`feat(monitor): 支持自定义关键字断言`
3. PR 要求：
   - 一个 PR 聚焦一件事，避免夹带无关格式化改动
   - 新功能附带必要的测试（Go 单测 / API 集成测试）
   - 涉及 UI 改动请附截图或录屏
   - 通过 `go vet ./...` 与 `npm run build` 无错误
4. 代码风格：
   - Go 代码遵循 `gofmt`；注释用中文或英文均可，但同文件保持一致
   - 前端遵循项目既有 ESLint/Prettier 配置

## 分支与发布

- `main`：始终可构建、可运行
- 发布版本打 `vX.Y.Z` 标签，附变更说明（Changelog）
- 维护策略见 [ROADMAP](ROADMAP.md)

## 许可

提交即表示你同意代码以 [Apache-2.0](LICENSE) 许可证随本项目分发。
