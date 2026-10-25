//go:build !windows

package main

import "github.com/gin-gonic/gin"

func isWindowsServiceMode() bool { return false }

func runAsWindowsService(addr string, router *gin.Engine) error { return nil }
