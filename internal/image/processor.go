package image

import (
	"fmt"

	"github.com/h2non/bimg"
)

type Size struct {
	Name    string
	Width   int
	Quality int
}

var Sizes = []Size{
	{Name: "original", Width: 4096, Quality: 90},
	{Name: "1920", Width: 1920, Quality: 90},
	{Name: "800", Width: 800, Quality: 90},
	{Name: "200", Width: 200, Quality: 90},
}

type ProcessResult struct {
	Name   string
	Data   []byte
	Width  int
	Height int
}

func Process(data []byte) ([]ProcessResult, error) {
	img := bimg.NewImage(data)

	// Get original dimensions
	size, err := img.Size()
	if err != nil {
		return nil, fmt.Errorf("get size: %w", err)
	}

	var results []ProcessResult

	for _, s := range Sizes {
		// Skip if original is smaller than target
		targetWidth := s.Width
		if size.Width < targetWidth {
			targetWidth = size.Width
		}

		// Re-encode to WebP (this strips all metadata and potential malicious content)
		processed, err := bimg.NewImage(data).Process(bimg.Options{
			Width:         targetWidth,
			Type:          bimg.WEBP,
			Quality:       s.Quality,
			StripMetadata: true,
		})
		if err != nil {
			return nil, fmt.Errorf("process %s: %w", s.Name, err)
		}

		// Get resulting dimensions
		resultImg := bimg.NewImage(processed)
		resultSize, err := resultImg.Size()
		if err != nil {
			return nil, fmt.Errorf("get result size %s: %w", s.Name, err)
		}

		results = append(results, ProcessResult{
			Name:   s.Name,
			Data:   processed,
			Width:  resultSize.Width,
			Height: resultSize.Height,
		})

		// If original was smaller than first target, we only need one version
		if size.Width <= Sizes[0].Width && s.Name == "original" {
			// Still generate smaller sizes from this
			continue
		}
	}

	return results, nil
}
