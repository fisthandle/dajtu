package image

import (
	"testing"

	"dajtu/internal/testutil"
)

func TestProcess_BasicJPEG(t *testing.T) {
	// Skip if libvips unavailable
	results, err := Process(testutil.SampleJPEG())
	if err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results returned")
	}

	// Verify we got expected sizes
	names := make(map[string]bool)
	for _, r := range results {
		names[r.Name] = true
	}

	expected := []string{"original", "thumb"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("missing result for size %q", name)
		}
	}
}

func TestProcessWithTransform_Rotation(t *testing.T) {
	// Skip if libvips unavailable
	if _, err := Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	p := NewProcessor()

	params := TransformParams{
		Rotation: 90,
	}

	results, err := p.ProcessWithTransform(testutil.SampleJPEG(), params)
	if err != nil {
		t.Fatalf("ProcessWithTransform: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results")
	}
}

func TestProcessWithTransform_Crop(t *testing.T) {
	if _, err := Process(testutil.SampleJPEG()); err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	p := NewProcessor()

	// Crop to 1x1 from 0,0
	params := TransformParams{
		CropX: 0,
		CropY: 0,
		CropW: 1,
		CropH: 1,
	}

	results, err := p.ProcessWithTransform(testutil.SampleJPEG(), params)
	if err != nil {
		t.Fatalf("ProcessWithTransform: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results")
	}
}

func TestTransformParams_HasTransforms(t *testing.T) {
	tests := []struct {
		name   string
		params TransformParams
		want   bool
	}{
		{"empty", TransformParams{}, false},
		{"rotation", TransformParams{Rotation: 90}, true},
		{"flipH", TransformParams{FlipH: true}, true},
		{"flipV", TransformParams{FlipV: true}, true},
		{"crop complete", TransformParams{CropW: 100, CropH: 100}, true},
		{"crop partial W only", TransformParams{CropW: 100}, false},
		{"crop partial H only", TransformParams{CropH: 100}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.params.HasTransforms(); got != tt.want {
				t.Errorf("HasTransforms() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProcess_PNG(t *testing.T) {
	results, err := Process(testutil.SamplePNG())
	if err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results returned for PNG")
	}
}

func TestProcess_WebP(t *testing.T) {
	results, err := Process(testutil.SampleWebP())
	if err != nil {
		t.Skipf("image processing unavailable: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("no results returned for WebP")
	}
}
