package media

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	v1 "github.com/WindowsSov8forUs/botgo-plus/openapi/v1"
	"golang.org/x/oauth2"
)

func TestChecksums(t *testing.T) {
	data := make([]byte, int(PrefixChecksumBytes)+20000)
	for i := range data {
		data[i] = byte(i*17 + 23)
	}
	whole, sha, prefix, err := Checksums(context.Background(), bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	wantWhole, wantSHA, wantPrefix := md5.Sum(data), sha1.Sum(data), md5.Sum(data[:PrefixChecksumBytes])
	if whole != hex.EncodeToString(wantWhole[:]) || sha != hex.EncodeToString(wantSHA[:]) || prefix != hex.EncodeToString(wantPrefix[:]) {
		t.Fatalf("checksums: md5=%s sha1=%s prefix=%s", whole, sha, prefix)
	}
}

func TestMultipartUpload(t *testing.T) {
	for _, scope := range []Scope{GroupScope, C2CScope} {
		t.Run(string(scope), func(t *testing.T) {
			data := []byte("hello world!")
			var mu sync.Mutex
			parts := make(map[int][]byte)
			confirmed := make(map[int]int64)
			var attempts atomic.Int32
			objects := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "PUT" {
					t.Errorf("object method=%s", r.Method)
				}
				index, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/part/"))
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					w.WriteHeader(400)
					return
				}
				if index == 0 && attempts.Add(1) == 1 {
					w.WriteHeader(503)
					return
				}
				mu.Lock()
				parts[index] = body
				mu.Unlock()
				w.WriteHeader(200)
			}))
			defer objects.Close()
			prefix := "/v2/groups/target"
			if scope == C2CScope {
				prefix = "/v2/users/target"
			}
			apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "QQBot fixture" || r.Method != "POST" {
					t.Errorf("API request=%s auth=%s", r.Method, r.Header.Get("Authorization"))
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case prefix + "/upload_prepare":
					var req dto.UploadPrepareRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					if req.FileSize != 12 || req.FileType != 4 || req.FileName != "fixture.txt" {
						t.Errorf("prepare=%+v", req)
					}
					response := dto.UploadPrepareResult{UploadID: "upload", BlockSize: 5, UploadConfig: dto.UploadConfig{Concurrency: 2, RetryTimeout: 10, RetryDelay: 1}}
					for _, index := range []int{2, 0, 1} {
						response.Parts = append(response.Parts, dto.UploadPart{Index: index, PresignedURL: fmt.Sprintf("%s/part/%d", objects.URL, index)})
					}
					_ = json.NewEncoder(w).Encode(response)
				case prefix + "/upload_part_finish":
					var req dto.UploadPartFinishRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					mu.Lock()
					body := append([]byte(nil), parts[req.PartIndex]...)
					confirmed[req.PartIndex] = int64(req.BlockSize)
					mu.Unlock()
					digest := md5.Sum(body)
					if int64(len(body)) != int64(req.BlockSize) || req.MD5 != hex.EncodeToString(digest[:]) || req.UploadID != "upload" {
						t.Errorf("confirmation=%+v", req)
					}
					_, _ = io.WriteString(w, `{}`)
				case prefix + "/files":
					var req dto.MediaUploadRequest
					if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
						t.Error(err)
					}
					mu.Lock()
					count := len(confirmed)
					mu.Unlock()
					if req.UploadID != "upload" || count != 3 {
						t.Errorf("merge=%+v confirmed=%d", req, count)
					}
					_, _ = io.WriteString(w, `{"file_uuid":"uuid","file_info":"opaque!file-info","ttl":100}`)
				default:
					t.Errorf("upload path=%s", r.URL.Path)
					w.WriteHeader(404)
				}
			}))
			defer apiServer.Close()
			source := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "fixture", TokenType: "QQBot"})
			api, err := v1.New("app", source, v1.WithBaseURL(apiServer.URL))
			if err != nil {
				t.Fatal(err)
			}
			uploader, err := NewUploader(api, Config{MaxConcurrency: 2, AllowHTTP: true})
			if err != nil {
				t.Fatal(err)
			}
			result, err := uploader.Upload(context.Background(), Target{Scope: scope, OpenID: "target"}, bytes.NewReader(data), int64(len(data)), 4, "fixture.txt")
			if err != nil {
				t.Fatalf("%v: %v", err, errors.Unwrap(err))
			}
			if result.FileInfo != "opaque!file-info" || result.TTL != 100 || attempts.Load() != 2 {
				t.Fatalf("result=%+v first-part attempts=%d", result, attempts.Load())
			}
			mu.Lock()
			defer mu.Unlock()
			joined := append(append(append([]byte(nil), parts[0]...), parts[1]...), parts[2]...)
			if confirmed[2] != 2 || !bytes.Equal(joined, data) {
				t.Fatalf("uploaded=%q last-part=%d", joined, confirmed[2])
			}
		})
	}
}
