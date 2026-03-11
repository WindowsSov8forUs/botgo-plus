package silk

import (
	"bytes"
	"crypto/md5"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

//go:embed exec/*
var silkCodecs embed.FS

const (
	headerAMR       = "#!AMR"
	headerSilk      = "\x02#!SILK_V3"
	headerSilkPlain = "#!SILK_V3"

	cachePath = "data/cache"
	scanLimit = 4 * 1024
)

// IsAMRorSILK reports whether data looks like AMR or SILK.
func IsAMRorSILK(file []byte) bool {
	return bytes.HasPrefix(file, []byte(headerAMR)) ||
		bytes.HasPrefix(file, []byte(headerSilk)) ||
		bytes.HasPrefix(file, []byte(headerSilkPlain))
}

// CheckAudio validates whether the stream looks like an audio payload.
func CheckAudio(readSeeker io.ReadSeeker) (string, bool) {
	contentType := scanType(readSeeker)
	return contentType, strings.HasPrefix(contentType, "audio/")
}

func scanType(readerSeeker io.ReadSeeker) string {
	_, _ = readerSeeker.Seek(0, io.SeekStart)
	defer readerSeeker.Seek(0, io.SeekStart)
	in := make([]byte, scanLimit)
	_, _ = readerSeeker.Read(in)
	return http.DetectContentType(in)
}

// EncodeSilk converts unsupported audio payloads to SILK.
func EncodeSilk(data []byte) ([]byte, error) {
	hash := md5.Sum(data)
	name := hex.EncodeToString(hash[:])
	return encode(data, name)
}

func encode(data []byte, name string) ([]byte, error) {
	if err := createDirectoryIfNotExist(cachePath); err != nil {
		return nil, fmt.Errorf("create audio cache directory: %w", err)
	}

	rawPath := filepath.Join(cachePath, name+".input")
	if err := os.WriteFile(rawPath, data, 0o600); err != nil {
		return nil, fmt.Errorf("write temporary audio: %w", err)
	}
	defer os.Remove(rawPath)

	sampleRate := 24000
	pcmPath := filepath.Join(cachePath, name+".pcm")
	defer os.Remove(pcmPath)

	ffmpegCmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", rawPath,
		"-f", "s16le",
		"-ar", strconv.Itoa(sampleRate),
		"-ac", "1",
		pcmPath,
	)
	if errors.Is(ffmpegCmd.Err, exec.ErrDot) {
		ffmpegCmd.Err = nil
	}
	ffmpegStderr := bytes.NewBuffer(nil)
	ffmpegCmd.Stderr = ffmpegStderr
	if err := ffmpegCmd.Run(); err != nil {
		return nil, fmt.Errorf("convert to pcm: %w (%s)", err, strings.TrimSpace(ffmpegStderr.String()))
	}

	codecPath, err := getSilkCodecPath()
	if err != nil {
		return nil, fmt.Errorf("get silk codec path: %w", err)
	}

	codecData, err := silkCodecs.ReadFile(codecPath)
	if err != nil {
		return nil, fmt.Errorf("read silk codec: %w", err)
	}

	codecFile, err := writeCodecTempFile(codecData)
	if err != nil {
		return nil, err
	}
	defer os.Remove(codecFile)

	silkPath := filepath.Join(cachePath, name+".silk")
	defer os.Remove(silkPath)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command(codecFile, "-i", pcmPath, "-o", silkPath, "-s", strconv.Itoa(sampleRate))
	} else {
		cmd = exec.Command(codecFile, "pts", "-i", pcmPath, "-o", silkPath, "-s", strconv.Itoa(sampleRate))
	}

	stderr := bytes.NewBuffer(nil)
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("encode silk: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	silkData, err := os.ReadFile(silkPath)
	if err != nil {
		return nil, fmt.Errorf("read silk file: %w", err)
	}
	return silkData, nil
}

func createDirectoryIfNotExist(path string) error {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return os.MkdirAll(path, os.ModePerm)
	}
	return err
}

func writeCodecTempFile(codecData []byte) (string, error) {
	pattern := "silk_codec*"
	if runtime.GOOS == "windows" {
		pattern += ".exe"
	}
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("create silk codec temp file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(codecData); err != nil {
		return "", fmt.Errorf("write silk codec temp file: %w", err)
	}
	if err := os.Chmod(file.Name(), 0o700); err != nil {
		return "", fmt.Errorf("chmod silk codec temp file: %w", err)
	}
	return file.Name(), nil
}

func getSilkCodecPath() (string, error) {
	var codecFileName string
	switch runtime.GOOS {
	case "windows":
		codecFileName = "silk_codec-windows.exe"
	case "linux":
		switch runtime.GOARCH {
		case "amd64":
			codecFileName = "silk_codec-linux-x64"
		case "arm64":
			codecFileName = "silk_codec-linux-arm64"
		default:
			return "", fmt.Errorf("unsupported linux architecture: %s", runtime.GOARCH)
		}
	case "darwin":
		switch runtime.GOARCH {
		case "amd64", "arm64":
			codecFileName = "silk_codec-macos"
		default:
			return "", fmt.Errorf("unsupported macos architecture: %s", runtime.GOARCH)
		}
	case "android":
		switch runtime.GOARCH {
		case "arm64":
			codecFileName = "silk_codec-android-arm64"
		case "x86":
			codecFileName = "silk_codec-android-x86"
		case "x86_64":
			codecFileName = "silk_codec-android-x86_64"
		default:
			return "", fmt.Errorf("unsupported android architecture: %s", runtime.GOARCH)
		}
	default:
		return "", fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return "exec/" + codecFileName, nil
}
