package main

// Guacamole 协议指令读写。
// 指令格式：<len>.<data>[,<len>.<data>...];  长度按 Unicode 字符数计（UTF-8 感知）。

import (
	"bufio"
	"errors"
	"io"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

var errGuacProto = errors.New("guacamole 协议错误")

const maxInstructionLen = 64 * 1024 * 1024 // 单元素上限 64MB

// guacReadInstruction 从 r 读取一条完整指令，返回 opcode 与参数列表。
func guacReadInstruction(r *bufio.Reader) (string, []string, error) {
	var args []string
	for {
		length := 0
		hasDigit := false
		for {
			b, err := r.ReadByte()
			if err != nil {
				return "", nil, err
			}
			if b >= '0' && b <= '9' {
				length = length*10 + int(b-'0')
				hasDigit = true
				continue
			}
			if b == '.' && hasDigit {
				break
			}
			return "", nil, errGuacProto
		}
		if length < 0 || length > maxInstructionLen {
			return "", nil, errGuacProto
		}
		data := make([]byte, 0, length*4)
		// 按字符数读：UTF-8 起始字节计 1，续接字节(10xxxxxx)不计
		for count := 0; count < length; {
			b, err := r.ReadByte()
			if err != nil {
				return "", nil, err
			}
			data = append(data, b)
			if b&0xC0 != 0x80 {
				count++
			}
		}
		args = append(args, string(data))
		sep, err := r.ReadByte()
		if err != nil {
			return "", nil, err
		}
		if sep == ';' {
			return args[0], args[1:], nil
		}
		if sep != ',' {
			return "", nil, errGuacProto
		}
	}
}

// guacEncode 按协议编码一条指令
func guacEncode(args ...string) string {
	var sb strings.Builder
	for i, a := range args {
		sb.WriteString(strconv.Itoa(utf8.RuneCountInString(a)))
		sb.WriteByte('.')
		sb.WriteString(a)
		if i == len(args)-1 {
			sb.WriteByte(';')
		} else {
			sb.WriteByte(',')
		}
	}
	return sb.String()
}

// guacWriter 并发安全的指令写出器（grdp 回调、stdin 循环、ticker 多方写入）。
// 每条指令写后立即 Flush，保证低位延迟。
type guacWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func newGuacWriter(w io.Writer) *guacWriter {
	return &guacWriter{w: w}
}

func (g *guacWriter) write(args ...string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, err := io.WriteString(g.w, guacEncode(args...))
	return err
}

// writeBatch 原子写出多条预编码指令（各指令已含结尾 ';'）。
// 同一 tick 的全部绘图指令合并为一次写入，使网关能将其并入单个
// WebSocket 帧，前端在同一个渲染帧内完成全部绘制，消除分块闪烁。
func (g *guacWriter) writeBatch(data string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	_, err := io.WriteString(g.w, data)
	return err
}
