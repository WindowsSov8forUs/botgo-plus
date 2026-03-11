package image

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	stdimage "image"
	"image/jpeg"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
)

const (
	headerJPG   = "\xFF\xD8"
	headerPNG   = "\x89PNG\r\n\x1a\n"
	headerGIF87 = "GIF87a"
	headerGIF89 = "GIF89a"

	cachePath = "data/cache"
	scanLimit = 4 * 1024
)

// IsGIFOrPNGOrJPG reports whether the binary data looks like GIF/PNG/JPG.
func IsGIFOrPNGOrJPG(file []byte) bool {
	if len(file) < 8 {
		return false
	}
	if bytes.HasPrefix(file, []byte(headerGIF87)) || bytes.HasPrefix(file, []byte(headerGIF89)) {
		return true
	}
	if bytes.HasPrefix(file, []byte(headerPNG)) {
		return true
	}
	return bytes.HasPrefix(file, []byte(headerJPG))
}

// CheckImage validates whether the stream looks like an image payload.
func CheckImage(readSeeker io.ReadSeeker) (string, bool) {
	contentType := scanType(readSeeker)
	return contentType, strings.HasPrefix(contentType, "image/")
}

func scanType(readerSeeker io.ReadSeeker) string {
	_, _ = readerSeeker.Seek(0, io.SeekStart)
	defer readerSeeker.Seek(0, io.SeekStart)
	in := make([]byte, scanLimit)
	_, _ = readerSeeker.Read(in)
	return http.DetectContentType(in)
}

// EncodeImage converts unsupported image formats to a compatible format.
func EncodeImage(data []byte) ([]byte, error) {
	hash := md5.Sum(data)
	name := hex.EncodeToString(hash[:])
	return encode(data, name)
}

func encode(data []byte, name string) ([]byte, error) {
	if err := createDirectoryIfNotExist(cachePath); err != nil {
		return nil, fmt.Errorf("create image cache directory: %w", err)
	}

	rawPath := filepath.Join(cachePath, name)
	if err := os.WriteFile(rawPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write temporary image: %w", err)
	}
	defer os.Remove(rawPath)

	img, err := imaging.Open(rawPath)
	if err != nil {
		return nil, fmt.Errorf("open temporary image: %w", err)
	}

	_, format, err := stdimage.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode image config: %w", err)
	}

	buffer := new(bytes.Buffer)
	if format == "bmp" {
		if err := jpeg.Encode(buffer, img, nil); err != nil {
			return nil, fmt.Errorf("convert bmp to jpg: %w", err)
		}
		return buffer.Bytes(), nil
	}

	if err := png.Encode(buffer, img); err != nil {
		return nil, fmt.Errorf("convert image to png: %w", err)
	}
	return buffer.Bytes(), nil
}

func createDirectoryIfNotExist(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return err
}
