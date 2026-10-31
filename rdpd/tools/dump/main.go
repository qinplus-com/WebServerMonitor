package main

// 位图样本取证：用 grdp Decompress 解压 rdpd-sample.bin，
// 按 4 种行序/通道序组合渲染 PNG，输出到指定目录，人工比对哪种正确。

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/x90skysn3k/grdp/core"
)

func render(data []byte, w, h, bpp int, flip, bgr bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for row := 0; row < h; row++ {
		srcRow := row
		if flip {
			srcRow = h - 1 - row
		}
		src := srcRow * w * bpp
		dst := row * img.Stride
		for col := 0; col < w; col++ {
			o := src + col*bpp
			var r, g, b byte
			if bgr {
				b, g, r = data[o], data[o+1], data[o+2]
			} else {
				r, g, b = data[o], data[o+1], data[o+2]
			}
			p := dst + col*4
			img.Pix[p], img.Pix[p+1], img.Pix[p+2], img.Pix[p+3] = r, g, b, 255
		}
	}
	return img
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("usage: dump <sample.bin> <sample.meta> <outdir>")
		os.Exit(1)
	}
	bin, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println("read bin:", err)
		os.Exit(1)
	}
	metaB, err := os.ReadFile(os.Args[2])
	if err != nil {
		fmt.Println("read meta:", err)
		os.Exit(1)
	}
	meta := string(metaB)
	get := func(key string) int {
		i := strings.Index(meta, key+"=")
		if i < 0 {
			return 0
		}
		rest := meta[i+len(key)+1:]
		if j := strings.IndexAny(rest, " )"); j >= 0 {
			rest = rest[:j]
		}
		v, _ := strconv.Atoi(rest)
		return v
	}
	w, h := get("w"), get("h")
	bpp := get("bppBits") / 8
	compress := strings.Contains(meta, "compress=true")

	data := bin
	if compress && len(bin) > 0 && bin[0] == 0x10 {
		data = core.Decompress(bin, w, h, bpp)
	} else {
		// rdpd 落盘的样本已是解压后数据（无 RDP6 头 0x10），直接渲染
		fmt.Println("input already decompressed, rendering raw")
	}
	fmt.Printf("w=%d h=%d bpp=%d compress=%v raw=%d decompressed=%d\n", w, h, bpp, compress, len(bin), len(data))
	if len(data) < w*h*bpp {
		fmt.Println("decompressed data too short")
		os.Exit(1)
	}

	out := os.Args[3]
	// 各字节位置均值 + 灰度图，定位通道真实位置
	for by := 0; by < bpp; by++ {
		sum := 0
		px := len(data) / bpp
		for p := 0; p < px; p++ {
			sum += int(data[p*bpp+by])
		}
		fmt.Printf("byte[%d] avg=%d\n", by, sum/px)
		g := image.NewGray(image.Rect(0, 0, w, h))
		for row := 0; row < h; row++ {
			for col := 0; col < w; col++ {
				g.Pix[row*g.Stride+col] = data[(row*w+col)*bpp+by]
			}
		}
		var buf bytes.Buffer
		_ = png.Encode(&buf, g)
		p := out + "\\rdpd-ch" + strconv.Itoa(by) + ".png"
		_ = os.WriteFile(p, buf.Bytes(), 0644)
		fmt.Println("saved", p)
	}
	for _, c := range []struct{ name string; flip, bgr bool }{
		{"flip-bgr", true, true},
		{"noflip-bgr", false, true},
		{"flip-rgb", true, false},
		{"noflip-rgb", false, false},
	} {
		var buf bytes.Buffer
		if err := png.Encode(&buf, render(data, w, h, bpp, c.flip, c.bgr)); err != nil {
			fmt.Println("png:", err)
			os.Exit(1)
		}
		p := out + "\\rdpd-view-" + c.name + ".png"
		if err := os.WriteFile(p, buf.Bytes(), 0644); err != nil {
			fmt.Println("write:", err)
			os.Exit(1)
		}
		fmt.Println("saved", p)
	}
}
