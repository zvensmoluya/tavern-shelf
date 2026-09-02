//go:build ignore

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zvensmoluya/tavern-shelf/internal/brand"
)

func main() {
	output := filepath.Join("build", "windows", "TavernShelf.ico")
	if len(os.Args) > 1 {
		output = os.Args[1]
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		panic(err)
	}
	sizes := brand.IconSizes
	images := make([][]byte, 0, len(sizes))
	for _, size := range sizes {
		encoded, err := brand.IconPNG(size)
		if err != nil {
			panic(err)
		}
		images = append(images, encoded)
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
