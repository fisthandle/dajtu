package image

import (
	"bytes"
	"testing"
)

func TestValidateAndDetect_JPEG(t *testing.T) {
	// Minimal JPEG header
	data := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46, 0x00, 0x01}
	// Pad to minimum size
	data = append(data, make([]byte, 100)...)

	format, result, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatJPEG {
		t.Errorf("format = %q, want %q", format, FormatJPEG)
	}
	if !bytes.Equal(result, data) {
		t.Error("returned data doesn't match input")
	}
}

func TestValidateAndDetect_PNG(t *testing.T) {
	data := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatPNG {
		t.Errorf("format = %q, want %q", format, FormatPNG)
	}
}

func TestValidateAndDetect_GIF(t *testing.T) {
	data := []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatGIF {
		t.Errorf("format = %q, want %q", format, FormatGIF)
	}
}

func TestValidateAndDetect_WebP(t *testing.T) {
	// RIFF....WEBP
	data := []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x00, 0x00, 0x00, 0x00, // size placeholder
		0x57, 0x45, 0x42, 0x50, // WEBP
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatWebP {
		t.Errorf("format = %q, want %q", format, FormatWebP)
	}
}

func TestValidateAndDetect_AVIF(t *testing.T) {
	// ftyp box with avif brand
	data := []byte{
		0x00, 0x00, 0x00, 0x00, // size
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x61, 0x76, 0x69, 0x66, // avif brand
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatAVIF {
		t.Errorf("format = %q, want %q", format, FormatAVIF)
	}
}

func TestValidateAndDetect_AVIF_Avis(t *testing.T) {
	// ftyp box with avis brand
	data := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x61, 0x76, 0x69, 0x73, // avis brand
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatAVIF {
		t.Errorf("format = %q, want %q", format, FormatAVIF)
	}
}

func TestValidateAndDetect_AVIF_Mif1(t *testing.T) {
	// ftyp box with mif1 brand
	data := []byte{
		0x00, 0x00, 0x00, 0x00,
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x6D, 0x69, 0x66, 0x31, // mif1 brand
	}
	data = append(data, make([]byte, 100)...)

	format, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if format != FormatAVIF {
		t.Errorf("format = %q, want %q", format, FormatAVIF)
	}
}

func TestValidateAndDetect_AVIF_WrongBrand(t *testing.T) {
	// ftyp box with non-AVIF brand (QuickTime)
	data := []byte{
		0x00, 0x00, 0x00, 0x18,
		0x66, 0x74, 0x79, 0x70, // ftyp
		0x71, 0x74, 0x20, 0x20, // "qt  " brand (QuickTime, not AVIF)
	}
	data = append(data, make([]byte, 20)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("should reject ftyp with non-AVIF brand, got error = %v", err)
	}
}

func TestValidateAndDetect_InvalidFormat(t *testing.T) {
	data := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	data = append(data, make([]byte, 100)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("error = %v, want ErrInvalidFormat", err)
	}
}

func TestValidateAndDetect_FileTooLarge(t *testing.T) {
	data := make([]byte, 1000)
	copy(data, []byte{0xFF, 0xD8, 0xFF}) // JPEG header

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 100)
	if err != ErrFileTooLarge {
		t.Errorf("error = %v, want ErrFileTooLarge", err)
	}
}

func TestValidateAndDetect_TooSmall(t *testing.T) {
	data := []byte{0xFF, 0xD8, 0xFF} // Only 3 bytes

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("error = %v, want ErrInvalidFormat (too small)", err)
	}
}

func TestValidateAndDetect_FakeExtension(t *testing.T) {
	// Executable disguised with JPEG-like data but wrong magic bytes
	data := []byte{0x4D, 0x5A, 0x90, 0x00, 0x03, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00} // MZ header (EXE)
	data = append(data, make([]byte, 100)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("should reject non-image files, got error = %v", err)
	}
}

func TestValidateAndDetect_RIFFNotWebP(t *testing.T) {
	// RIFF file but not WebP (e.g., WAV)
	data := []byte{
		0x52, 0x49, 0x46, 0x46, // RIFF
		0x00, 0x00, 0x00, 0x00,
		0x57, 0x41, 0x56, 0x45, // WAVE (not WEBP)
	}
	data = append(data, make([]byte, 100)...)

	_, _, err := ValidateAndDetect(bytes.NewReader(data), 1024*1024)
	if err != ErrInvalidFormat {
		t.Errorf("should reject RIFF non-WebP, got error = %v", err)
	}
}

func TestDetectFormat_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected Format
	}{
		{
			name:     "JPEG",
			data:     append([]byte{0xFF, 0xD8, 0xFF, 0xE0}, make([]byte, 20)...),
			expected: FormatJPEG,
		},
		{
			name:     "PNG",
			data:     append([]byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, make([]byte, 20)...),
			expected: FormatPNG,
		},
		{
			name:     "GIF87a",
			data:     append([]byte{0x47, 0x49, 0x46, 0x38, 0x37, 0x61}, make([]byte, 20)...),
			expected: FormatGIF,
		},
		{
			name:     "GIF89a",
			data:     append([]byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61}, make([]byte, 20)...),
			expected: FormatGIF,
		},
		{
			name:     "Unknown",
			data:     make([]byte, 20),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFormat(tt.data)
			if result != tt.expected {
				t.Errorf("detectFormat() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestValidateAndDetect_ReadError(t *testing.T) {
	_, _, err := ValidateAndDetect(&failReader{}, 1024)
	if err == nil {
		t.Error("expected error from failing reader")
	}
}

type failReader struct{}

func (f *failReader) Read(p []byte) (n int, err error) {
	return 0, bytes.ErrTooLarge
}
