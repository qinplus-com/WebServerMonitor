//go:build opensource

package license

// 社区版（开源构建）标记：使用 `go build -tags opensource` 编译时生效。
// 社区版不启用授权校验，无服务器数量与使用期限限制，遵循 Apache-2.0 许可证分发。
const communityEdition = true
