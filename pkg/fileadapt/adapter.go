package fileadapt

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/tencent-connect/botgo/dto"
	imagepkg "github.com/tencent-connect/botgo/pkg/image"
	mp4pkg "github.com/tencent-connect/botgo/pkg/mp4"
	silkpkg "github.com/tencent-connect/botgo/pkg/silk"
)

const (
	// FileTypeImage indicates image rich media payload.
	FileTypeImage uint64 = 1
	// FileTypeVideo indicates video rich media payload.
	FileTypeVideo uint64 = 2
	// FileTypeAudio indicates audio rich media payload.
	FileTypeAudio uint64 = 3
)

// AdaptAPIMessage adapts rich media file payload to platform-supported formats.
func AdaptAPIMessage(msg dto.APIMessage) (dto.APIMessage, error) {
	switch rich := msg.(type) {
	case *dto.RichMediaMessage:
		if err := adaptRichMediaMessage(rich); err != nil {
			return msg, err
		}
		return rich, nil
	case dto.RichMediaMessage:
		m := rich
		if err := adaptRichMediaMessage(&m); err != nil {
			return msg, err
		}
		return m, nil
	default:
		return msg, nil
	}
}

func adaptRichMediaMessage(msg *dto.RichMediaMessage) error {
	if msg == nil || msg.FileData == "" {
		return nil
	}

	data, err := decodeBase64(msg.FileData)
	if err != nil {
		return fmt.Errorf("decode rich media file_data: %w", err)
	}

	adapted, err := adaptByFileType(msg.FileType, data)
	if err != nil {
		return err
	}
	msg.FileData = base64.StdEncoding.EncodeToString(adapted)
	return nil
}

func adaptByFileType(fileType uint64, data []byte) ([]byte, error) {
	switch fileType {
	case FileTypeImage:
		if imagepkg.IsGIFOrPNGOrJPG(data) {
			return data, nil
		}
		if _, ok := imagepkg.CheckImage(bytes.NewReader(data)); !ok {
			return nil, fmt.Errorf("file_type=1 but payload is not a valid image")
		}
		encoded, err := imagepkg.EncodeImage(data)
		if err != nil {
			return nil, fmt.Errorf("convert image to supported format: %w", err)
		}
		return encoded, nil
	case FileTypeVideo:
		if mp4pkg.IsMP4(data) {
			return data, nil
		}
		if _, ok := mp4pkg.CheckVideo(bytes.NewReader(data)); !ok {
			return nil, fmt.Errorf("file_type=2 but payload is not a valid video")
		}
		encoded, err := mp4pkg.EncodeMP4(data)
		if err != nil {
			return nil, fmt.Errorf("convert video to mp4: %w", err)
		}
		return encoded, nil
	case FileTypeAudio:
		if silkpkg.IsAMRorSILK(data) {
			return data, nil
		}
		if _, ok := silkpkg.CheckAudio(bytes.NewReader(data)); !ok {
			return nil, fmt.Errorf("file_type=3 but payload is not a valid audio")
		}
		encoded, err := silkpkg.EncodeSilk(data)
		if err != nil {
			return nil, fmt.Errorf("convert audio to silk: %w", err)
		}
		return encoded, nil
	default:
		return data, nil
	}
}

func decodeBase64(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty file_data")
	}
	if comma := strings.IndexByte(raw, ','); comma > 0 && strings.Contains(raw[:comma], "base64") {
		raw = raw[comma+1:]
	}

	if data, err := base64.StdEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	if data, err := base64.RawStdEncoding.DecodeString(raw); err == nil {
		return data, nil
	}
	data, err := base64.URLEncoding.DecodeString(raw)
	if err != nil {
		return nil, err
	}
	return data, nil
}
