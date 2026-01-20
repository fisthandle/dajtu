package image

import (
	"bytes"
	"errors"
	"io"
)

var (
	ErrInvalidFormat = errors.New("invalid image format")
	ErrFileTooLarge  = errors.New("file too large")
)

type Format string

const (
	FormatJPEG Format = "image/jpeg"
	FormatPNG  Format = "image/png"
	FormatGIF  Format = "image/gif"
	FormatWebP Format = "image/webp"
	FormatAVIF Format = "image/avif"
)

var magicBytes = map[Format][]byte{
	FormatJPEG: {0xFF, 0xD8, 0xFF},
	FormatPNG:  {0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
	FormatGIF:  {0x47, 0x49, 0x46, 0x38}, // GIF8
	FormatWebP: {0x52, 0x49, 0x46, 0x46}, // RIFF (need to check WEBP at offset 8)
	FormatAVIF: {0x00, 0x00, 0x00},       // ftyp box (need deeper check)
}

func ValidateAndDetect(r io.Reader, maxSize int64) (Format, []byte, error) {
	// Read entire file with size limit
	limited := io.LimitReader(r, maxSize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return "", nil, err
	}
	if int64(len(data)) > maxSize {
		return "", nil, ErrFileTooLarge
	}
	if len(data) < 12 {
		return "", nil, ErrInvalidFormat
	}

	format := detectFormat(data)
	if format == "" {
		return "", nil, ErrInvalidFormat
	}

	return format, data, nil
}

func detectFormat(data []byte) Format {
	// JPEG
	if bytes.HasPrefix(data, magicBytes[FormatJPEG]) {
		return FormatJPEG
	}

	// PNG
	if bytes.HasPrefix(data, magicBytes[FormatPNG]) {
		return FormatPNG
	}

	// GIF
	if bytes.HasPrefix(data, magicBytes[FormatGIF]) {
		return FormatGIF
	}

	// WebP: RIFF....WEBP
	if bytes.HasPrefix(data, magicBytes[FormatWebP]) && len(data) >= 12 {
		if bytes.Equal(data[8:12], []byte("WEBP")) {
			return FormatWebP
		}
	}

	// AVIF: ftyp box with avif/avis brand
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) {
		brand := string(data[8:12])
		if brand == "avif" || brand == "avis" || brand == "mif1" {
			return FormatAVIF
		}
	}

	return ""
}
