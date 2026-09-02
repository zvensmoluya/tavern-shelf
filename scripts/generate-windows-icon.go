//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	output := filepath.Join("build", "windows", "TavernShelf.ico")
	if len(os.Args) > 1 {
		output = os.Args[1]
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		panic(err)
	}
	sizes := []int{16, 24, 32, 48, 64, 128, 256}
	images := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, renderIcon(size)); err != nil {
			panic(err)
		}
		images = append(images, encoded.Bytes())
	}
	var icon bytes.Buffer
	_ = binary.Write(&icon, binary.LittleEndian, uint16(0))
	_ = binary.Write(&icon, binary.LittleEndian, uint16(1))
	_ = binary.Write(&icon, binary.LittleEndian, uint16(len(images)))
	offset := uint32(6 + 16*len(images))
	for index, encoded := range images {
		dimension := byte(sizes[index])
		if sizes[index] == 256 {
			dimension = 0
		}
		icon.WriteByte(dimension)
		icon.WriteByte(dimension)
		icon.WriteByte(0)
		icon.WriteByte(0)
		_ = binary.Write(&icon, binary.LittleEndian, uint16(1))
		_ = binary.Write(&icon, binary.LittleEndian, uint16(32))
		_ = binary.Write(&icon, binary.LittleEndian, uint32(len(encoded)))
		_ = binary.Write(&icon, binary.LittleEndian, offset)
		offset += uint32(len(encoded))
	}
	for _, encoded := range images {
		_, _ = icon.Write(encoded)
	}
	if err := os.WriteFile(output, icon.Bytes(), 0o644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %s\n", output)
}

func renderIcon(size int) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, size, size))
	dark := color.RGBA{R: 21, G: 24, B: 29, A: 255}
	border := color.RGBA{R: 54, G: 59, B: 69, A: 255}
	light := color.RGBA{R: 232, G: 235, B: 239, A: 255}
	muted := color.RGBA{R: 104, G: 112, B: 125, A: 255}
	for y := scaled(4, size); y < scaled(60, size); y++ {
		for x := scaled(4, size); x < scaled(60, size); x++ {
			dx := max(0, max(scaled(19, size)-x, x-scaled(44, size)))
			dy := max(0, max(scaled(19, size)-y, y-scaled(44, size)))
			radius := max(1, scaled(15, size))
			if dx*dx+dy*dy <= radius*radius {
				canvas.SetRGBA(x, y, dark)
				if x < scaled(6, size) || x >= scaled(58, size) || y < scaled(6, size) || y >= scaled(58, size) {
					canvas.SetRGBA(x, y, border)
				}
			}
		}
	}
	fill(canvas, 17, 18, 30, 47, light)
	fill(canvas, 34, 18, 47, 47, light)
	fill(canvas, 21, 23, 26, 42, muted)
	fill(canvas, 38, 23, 43, 42, muted)
	fill(canvas, 14, 49, 50, 51, muted)
	return canvas
}

func fill(canvas *image.RGBA, left, top, right, bottom int, value color.RGBA) {
	size := canvas.Bounds().Dx()
	for y := scaled(top, size); y < scaled(bottom, size); y++ {
		for x := scaled(left, size); x < scaled(right, size); x++ {
			canvas.SetRGBA(x, y, value)
		}
	}
}

func scaled(value, size int) int {
	return (value*size + 32) / 64
}
