package mp4

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	cachePath = "data/cache"
	scanLimit = 4 * 1024
)

// IsMP4 reports whether data appears to be MP4 data.
func IsMP4(file []byte) bool {
	if len(file) < 12 {
		return false
	}
	return bytes.Equal(file[4:8], []byte("ftyp"))
}

// CheckVideo validates whether the stream looks like a video payload.
func CheckVideo(readSeeker io.ReadSeeker) (string, bool) {
	contentType := scanType(readSeeker)
	return contentType, strings.HasPrefix(contentType, "video/")
}

func scanType(readerSeeker io.ReadSeeker) string {
	_, _ = readerSeeker.Seek(0, io.SeekStart)
	defer readerSeeker.Seek(0, io.SeekStart)
	in := make([]byte, scanLimit)
	_, _ = readerSeeker.Read(in)
	return http.DetectContentType(in)
}

// EncodeMP4 converts unsupported video data to MP4(H264/AAC).
func EncodeMP4(data []byte) ([]byte, error) {
	hash := md5.Sum(data)
	name := hex.EncodeToString(hash[:])
	return encode(data, name)
}

func encode(data []byte, name string) ([]byte, error) {
	if err := createDirectoryIfNotExist(cachePath); err != nil {
		return nil, fmt.Errorf("create video cache directory: %w", err)
	}

	rawPath := filepath.Join(cachePath, name)
	if err := os.WriteFile(rawPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write temporary video: %w", err)
	}
	defer os.Remove(rawPath)

	mp4Path := filepath.Join(cachePath, name+".mp4")
	defer os.Remove(mp4Path)

	cmd := exec.Command("ffmpeg", "-y", "-i", rawPath, "-vcodec", "libx264", "-acodec", "aac", mp4Path)
	if errors.Is(cmd.Err, exec.ErrDot) {
		cmd.Err = nil
	}
	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("convert to mp4: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	video, err := os.ReadFile(mp4Path)
	if err != nil {
		return nil, fmt.Errorf("read converted mp4: %w", err)
	}
	return video, nil
}

func createDirectoryIfNotExist(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return err
}
