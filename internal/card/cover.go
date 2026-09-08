package card

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"image/png"
)

// Pixel identity ignores PNG compression and card metadata. Bound decoding
// memory independently of file size; oversized covers remain displayable but
// unclassified, and their original files are still collected.
func coverFingerprint(raw []byte) string {
	config, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 16_000_000 {
		return ""
	}
	image, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return ""
	}
	hash := sha256.New()
	var dimensions [8]byte
	binary.BigEndian.PutUint32(dimensions[:4], uint32(config.Width))
	binary.BigEndian.PutUint32(dimensions[4:], uint32(config.Height))
	hash.Write(dimensions[:])
	row := make([]byte, config.Width*8)
	for y := 0; y < config.Height; y++ {
		for x := 0; x < config.Width; x++ {
			r, g, b, a := image.At(x, y).RGBA()
			for index, value := range []uint32{r, g, b, a} {
				binary.BigEndian.PutUint16(row[x*8+index*2:], uint16(value))
			}
		}
		hash.Write(row)
	}
	return hex.EncodeToString(hash.Sum(nil))
}
