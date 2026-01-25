package image

import (
	"fmt"

	"github.com/h2non/bimg"
)

func GetSize(data []byte) (int, int, error) {
	size, err := bimg.NewImage(data).Size()
	if err != nil {
		return 0, 0, fmt.Errorf("get size: %w", err)
	}
	return size.Width, size.Height, nil
}

func ResizeToWidth(data []byte, width int) ([]byte, error) {
	opts := bimg.Options{
		Width:         width,
		Type:          bimg.WEBP,
		Quality:       90,
		StripMetadata: true,
	}

	processed, err := bimg.NewImage(data).Process(opts)
	if err != nil {
		return nil, fmt.Errorf("resize to %d: %w", width, err)
	}
	return processed, nil
}
