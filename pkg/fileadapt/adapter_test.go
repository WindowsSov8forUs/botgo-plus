package fileadapt

import (
	"encoding/base64"
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
)

func TestAdaptAPIMessage_Image_NoConvert(t *testing.T) {
	raw := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00}
	msg := &dto.RichMediaMessage{
		FileType: FileTypeImage,
		FileData: base64.StdEncoding.EncodeToString(raw),
	}

	adapted, err := AdaptAPIMessage(msg)
	if err != nil {
		t.Fatalf("AdaptAPIMessage() error = %v", err)
	}

	after := adapted.(*dto.RichMediaMessage)
	if after.FileData != msg.FileData {
		t.Fatalf("expected no conversion, got different file_data")
	}
}

func TestAdaptAPIMessage_Video_NoConvert(t *testing.T) {
	raw := []byte{
		0x00, 0x00, 0x00, 0x20,
		'f', 't', 'y', 'p',
		'i', 's', 'o', 'm',
		0x00, 0x00, 0x00, 0x00,
	}
	msg := &dto.RichMediaMessage{
		FileType: FileTypeVideo,
		FileData: base64.StdEncoding.EncodeToString(raw),
	}

	adapted, err := AdaptAPIMessage(msg)
	if err != nil {
		t.Fatalf("AdaptAPIMessage() error = %v", err)
	}

	after := adapted.(*dto.RichMediaMessage)
	if after.FileData != msg.FileData {
		t.Fatalf("expected no conversion, got different file_data")
	}
}

func TestAdaptAPIMessage_Audio_NoConvert(t *testing.T) {
	raw := []byte{0x02, '#', '!', 'S', 'I', 'L', 'K', '_', 'V', '3', 0x00}
	msg := &dto.RichMediaMessage{
		FileType: FileTypeAudio,
		FileData: base64.StdEncoding.EncodeToString(raw),
	}

	adapted, err := AdaptAPIMessage(msg)
	if err != nil {
		t.Fatalf("AdaptAPIMessage() error = %v", err)
	}

	after := adapted.(*dto.RichMediaMessage)
	if after.FileData != msg.FileData {
		t.Fatalf("expected no conversion, got different file_data")
	}
}

func TestAdaptAPIMessage_BadBase64(t *testing.T) {
	msg := &dto.RichMediaMessage{
		FileType: FileTypeImage,
		FileData: "***",
	}

	_, err := AdaptAPIMessage(msg)
	if err == nil {
		t.Fatalf("expected decode error")
	}
}
