package brand

import (
	"embed"
	"fmt"
)

var IconSizes = []int{16, 24, 32, 48, 64, 128, 256}

//go:embed icons/*.png
var icons embed.FS

func IconPNG(size int) ([]byte, error) {
	data, err := icons.ReadFile(fmt.Sprintf("icons/app-icon-%d.png", size))
	if err != nil {
		return nil, fmt.Errorf("read %dpx brand icon: %w", size, err)
	}
	return data, nil
}

func BrandMarkPNG() ([]byte, error) {
	data, err := icons.ReadFile("icons/brand-mark-256.png")
	if err != nil {
		return nil, fmt.Errorf("read frontend brand mark: %w", err)
	}
	return data, nil
}
