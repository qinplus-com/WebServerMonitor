package main

// 生成默认箭头光标 PNG（grdp 暂不处理服务器指针更新，一期使用静态箭头）。
// 黑色箭头 + 白色描边，热点在左上角 (0,0)。

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
)

const cursorW = 12
const cursorH = 19

// 箭头多边形顶点
var arrowPoly = [][2]int{
	{0, 0}, {0, 13}, {3, 10}, {5, 15}, {7, 14}, {5, 9}, {9, 9},
}

func pointInPoly(x, y int, poly [][2]int) bool {
	inside := false
	j := len(poly) - 1
	for i := 0; i < len(poly); i++ {
		xi, yi := poly[i][0], poly[i][1]
		xj, yj := poly[j][0], poly[j][1]
		if (yi > y) != (yj > y) && x < (xj-xi)*(y-yi)/(yj-yi)+xi {
			inside = !inside
		}
		j = i
	}
	return inside
}

// buildCursorPNG 渲染箭头并编码为 PNG
func buildCursorPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, cursorW, cursorH))
	black := make([][]bool, cursorH)
	for y := range black {
		black[y] = make([]bool, cursorW)
	}
	for y := 0; y < cursorH; y++ {
		for x := 0; x < cursorW; x++ {
			if pointInPoly(x, y, arrowPoly) {
				black[y][x] = true
			}
		}
	}
	// 白色描边：黑色像素的 4 邻域
	white := make([][]bool, cursorH)
	for y := range white {
		white[y] = make([]bool, cursorW)
	}
	for y := 0; y < cursorH; y++ {
		for x := 0; x < cursorW; x++ {
			if black[y][x] {
				continue
			}
			if (x > 0 && black[y][x-1]) || (x < cursorW-1 && black[y][x+1]) ||
				(y > 0 && black[y-1][x]) || (y < cursorH-1 && black[y+1][x]) {
				white[y][x] = true
			}
		}
	}
	for y := 0; y < cursorH; y++ {
		for x := 0; x < cursorW; x++ {
			i := img.PixOffset(x, y)
			switch {
			case black[y][x]:
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 20, 20, 20, 255
			case white[y][x]:
				img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 255, 255, 255, 255
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

var cursorPNGBase64 = base64.StdEncoding.EncodeToString(buildCursorPNG())
