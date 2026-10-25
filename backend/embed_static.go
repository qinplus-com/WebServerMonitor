package main

import "embed"

// 前端构建产物（backend/static）在编译时内嵌进可执行文件，
// 交付时单个 exe 即可运行，无需外部 static 目录。
//
//go:embed all:static
var staticEmbed embed.FS
