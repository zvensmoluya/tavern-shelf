package brand

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbeddedIcons(t *testing.T) {
	for _, size := range IconSizes {
		data, err := IconPNG(size)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(data, []byte("\x89PNG\r\n\x1a\n")) {
			t.Fatalf("%dpx icon is not a PNG", size)
		}
		image, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode %dpx icon: %v", size, err)
		}
		if image.Bounds().Dx() != size || image.Bounds().Dy() != size {
			t.Fatalf("%dpx icon has dimensions %s", size, image.Bounds())
		}
	}
}

func TestFrontendIconMatchesEmbeddedIcon(t *testing.T) {
	want, err := BrandMarkPNG()
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join("..", "..", "frontend", "public", "brand-mark.png"))
	if err != nil {
		t.Fatalf("read frontend icon: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("frontend icon does not match the embedded 256px brand icon")
	}
}
