package image

import (
	"fmt"

	"github.com/h2non/bimg"
)

type Size struct {
	Name    string
	Width   int
	Height  int
	Quality int
}

var Sizes = []Size{
	{Name: "original", Width: 4096, Height: 0, Quality: 90},
	{Name: "1920", Width: 1920, Height: 0, Quality: 90},
	{Name: "800", Width: 800, Height: 0, Quality: 90},
	{Name: "200", Width: 200, Height: 0, Quality: 90},
	{Name: "thumb", Width: 150, Height: 150, Quality: 85},
}

type ProcessResult struct {
	Name   string
	Data   []byte
	Width  int
	Height int
}

type TransformParams struct {
	Rotation int
	FlipH    bool
	FlipV    bool
	CropX    int
	CropY    int
	CropW    int
	CropH    int
}

func (p TransformParams) HasTransforms() bool {
	return p.Rotation != 0 || p.FlipH || p.FlipV || (p.CropW > 0 && p.CropH > 0)
}

type Processor struct{}

func NewProcessor() *Processor {
	return &Processor{}
}

func Process(data []byte) ([]ProcessResult, error) {
	return processVariants(data, false)
}

func (p *Processor) ProcessWithTransform(data []byte, params TransformParams) ([]ProcessResult, error) {
	if !params.HasTransforms() {
		return Process(data)
	}

	opts := bimg.Options{
		StripMetadata: true,
	}

	rotation := ((params.Rotation % 360) + 360) % 360
	switch rotation {
	case 90:
		opts.Rotate = bimg.D90
	case 180:
		opts.Rotate = bimg.D180
	case 270:
		opts.Rotate = bimg.D270
	}

	if params.FlipH {
		opts.Flip = true
	}
	if params.FlipV {
		opts.Flop = true
	}

	cropX := params.CropX
	cropY := params.CropY
	cropW := params.CropW
	cropH := params.CropH
	if cropX < 0 {
		cropX = 0
	}
	if cropY < 0 {
		cropY = 0
	}
	if cropW < 0 {
		cropW = 0
	}
	if cropH < 0 {
		cropH = 0
	}
	if cropW > 0 && cropH > 0 {
		opts.Top = cropY
		opts.Left = cropX
		opts.AreaWidth = cropW
		opts.AreaHeight = cropH
	}

	transformed, err := bimg.NewImage(data).Process(opts)
	if err != nil {
		return nil, fmt.Errorf("transform: %w", err)
	}

	return processVariants(transformed, true)
}

func processVariants(data []byte, noAutoRotate bool) ([]ProcessResult, error) {
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
		opts := bimg.Options{
			Width:         targetWidth,
			Type:          bimg.WEBP,
			Quality:       s.Quality,
			StripMetadata: true,
			NoAutoRotate:  noAutoRotate,
		}

		if s.Height > 0 {
			opts.Width = s.Width
			opts.Height = s.Height
			opts.Crop = true
			opts.Gravity = bimg.GravityCentre
		}

		processed, err := bimg.NewImage(data).Process(opts)
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
