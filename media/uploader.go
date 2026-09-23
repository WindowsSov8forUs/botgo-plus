// Package media uploads native QQ media without converting messages or sending them automatically.
package media

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	v1 "github.com/WindowsSov8forUs/botgo-plus/openapi/v1"
)

// PrefixChecksumBytes is the exact prefix length specified by QQ for md5_10m.
const PrefixChecksumBytes int64 = 10002432

type Scope string

const (
	GroupScope Scope = "group"
	C2CScope   Scope = "c2c"
)

type Target struct {
	Scope  Scope
	OpenID string
}

// Config contains local resource limits, not a statement of QQ account/platform limits.
type Config struct {
	MaxConcurrency int
	MaxFileBytes   int64
	MaxPUTAttempts int
	PUTTimeout     time.Duration
	MaxRetryWindow time.Duration
	// HTTPClient must be credential-free. Never pass a QQ-authenticated client here.
	HTTPClient *http.Client
	// AllowHTTP is intended for controlled test servers. Production signed uploads use HTTPS.
	AllowHTTP bool
}

type Uploader struct {
	api    *v1.Client
	config Config
	client *http.Client
}

func NewUploader(api *v1.Client, config Config) (*Uploader, error) {
	if api == nil {
		return nil, errors.New("QQ API client is required")
	}
	if config.MaxConcurrency == 0 {
		config.MaxConcurrency = 4
	}
	if config.MaxFileBytes == 0 {
		config.MaxFileBytes = 256 * 1024 * 1024
	}
	if config.MaxPUTAttempts == 0 {
		config.MaxPUTAttempts = 3
	}
	if config.PUTTimeout == 0 {
		config.PUTTimeout = 30 * time.Second
	}
	if config.MaxRetryWindow == 0 {
		config.MaxRetryWindow = 2 * time.Minute
	}
	if config.MaxConcurrency < 1 || config.MaxConcurrency > 64 || config.MaxFileBytes < 1 || config.MaxPUTAttempts < 1 || config.MaxPUTAttempts > 20 || config.PUTTimeout <= 0 || config.MaxRetryWindow <= 0 {
		return nil, errors.New("invalid media upload limits")
	}
	client := &http.Client{}
	if config.HTTPClient != nil {
		copy := *config.HTTPClient
		client = &copy
	}
	client.Jar = nil
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	client.Timeout = 0 // Each PUT has an explicit context deadline.
	return &Uploader{api: api, config: config, client: client}, nil
}

// UploadError preserves the stage and upload ID for diagnosis. Error avoids printing signed URLs.
// A failed upload may have confirmed some parts; no automatic restart or resend is performed.
type UploadError struct {
	Stage     string
	UploadID  string
	PartIndex int
	Cause     error
}

func (e *UploadError) Error() string {
	return fmt.Sprintf("QQ media upload failed at %s (part %d)", e.Stage, e.PartIndex)
}
func (e *UploadError) Unwrap() error { return e.Cause }

type checkedReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r checkedReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(p)
}

// Checksums reads a stable ReaderAt with bounded memory and honors context cancellation.
func Checksums(ctx context.Context, source io.ReaderAt, size int64) (wholeMD5, wholeSHA1, prefixMD5 string, err error) {
	if source == nil || size <= 0 {
		return "", "", "", errors.New("non-empty media source is required")
	}
	md, sh := md5.New(), sha1.New()
	buffer := make([]byte, 64*1024)
	var copied int64
	if copied, err = io.CopyBuffer(io.MultiWriter(md, sh), checkedReader{ctx, io.NewSectionReader(source, 0, size)}, buffer); err != nil {
		return
	}
	if copied != size {
		err = io.ErrUnexpectedEOF
		return
	}
	prefix := md5.New()
	length := size
	if length > PrefixChecksumBytes {
		length = PrefixChecksumBytes
	}
	if copied, err = io.CopyBuffer(prefix, checkedReader{ctx, io.NewSectionReader(source, 0, length)}, buffer); err != nil {
		return
	}
	if copied != length {
		err = io.ErrUnexpectedEOF
		return
	}
	return hex.EncodeToString(md.Sum(nil)), hex.EncodeToString(sh.Sum(nil)), hex.EncodeToString(prefix.Sum(nil)), nil
}

type chunk struct {
	part   dto.UploadPart
	offset int64
	size   int64
}

func uploadLayout(plan *dto.UploadPrepareResult, size int64) ([]chunk, error) {
	if plan == nil || plan.UploadID == "" || len(plan.Parts) == 0 || len(plan.Parts) > 65536 {
		return nil, errors.New("invalid or unsupported upload plan")
	}
	parts := append([]dto.UploadPart(nil), plan.Parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].Index < parts[j].Index })
	result := make([]chunk, 0, len(parts))
	var offset int64
	for index, part := range parts {
		if part.Index != index || part.PresignedURL == "" || offset >= size {
			return nil, errors.New("non-contiguous upload parts")
		}
		length := int64(part.BlockSize)
		if length == 0 {
			length = int64(plan.BlockSize)
		}
		if length <= 0 {
			return nil, errors.New("invalid upload block size")
		}
		if remaining := size - offset; length > remaining {
			length = remaining
		}
		result = append(result, chunk{part: part, offset: offset, size: length})
		offset += length
	}
	if offset != size {
		return nil, errors.New("upload plan does not cover the entire source")
	}
	return result, nil
}

// Upload uploads a stable, concurrently readable source, but does not send a chat message.
// Applications must keep the file unchanged until Upload returns.
func (u *Uploader) Upload(ctx context.Context, target Target, source io.ReaderAt, size int64, fileType int, fileName string) (*dto.MediaUploadResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if target.OpenID == "" || (target.Scope != GroupScope && target.Scope != C2CScope) {
		return nil, errors.New("invalid QQ upload target")
	}
	if source == nil || size <= 0 || size > u.config.MaxFileBytes || fileType < 1 || fileType > 4 || fileName == "" {
		return nil, errors.New("invalid media source or local size limit exceeded")
	}
	md, sh, prefix, err := Checksums(ctx, source, size)
	if err != nil {
		return nil, &UploadError{Stage: "checksum", PartIndex: -1, Cause: err}
	}
	request := &dto.UploadPrepareRequest{FileType: fileType, FileSize: dto.DecimalInt64(size), FileName: fileName, MD5: md, SHA1: sh, MD5Prefix: prefix}
	var plan *dto.UploadPrepareResult
	if target.Scope == GroupScope {
		plan, _, err = u.api.PrepareGroupUpload(ctx, target.OpenID, request)
	} else {
		plan, _, err = u.api.PrepareC2CUpload(ctx, target.OpenID, request)
	}
	if err != nil {
		return nil, &UploadError{Stage: "prepare", PartIndex: -1, Cause: err}
	}
	chunks, err := uploadLayout(plan, size)
	if err != nil {
		return nil, &UploadError{Stage: "layout", UploadID: plan.UploadID, PartIndex: -1, Cause: err}
	}
	for _, item := range chunks {
		if err := u.validatePUTURL(item.part.PresignedURL); err != nil {
			return nil, &UploadError{Stage: "signed-url", UploadID: plan.UploadID, PartIndex: item.part.Index, Cause: err}
		}
	}
	concurrency := plan.UploadConfig.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > u.config.MaxConcurrency {
		concurrency = u.config.MaxConcurrency
	}
	if concurrency > len(chunks) {
		concurrency = len(chunks)
	}
	workCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	jobs := make(chan chunk)
	failures := make(chan error, 1)
	var workers sync.WaitGroup
	for n := 0; n < concurrency; n++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				if err := workCtx.Err(); err != nil {
					return
				}
				if err := u.uploadChunk(workCtx, target, source, plan, item); err != nil {
					select {
					case failures <- err:
					default:
					}
					cancel()
					return
				}
			}
		}()
	}
feed:
	for _, item := range chunks {
		select {
		case jobs <- item:
		case <-workCtx.Done():
			break feed
		}
	}
	close(jobs)
	workers.Wait()
	select {
	case err := <-failures:
		return nil, err
	default:
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	merge := &dto.MediaUploadRequest{UploadID: plan.UploadID}
	var result *dto.MediaUploadResult
	if target.Scope == GroupScope {
		result, _, err = u.api.UploadGroupFile(ctx, target.OpenID, merge)
	} else {
		result, _, err = u.api.UploadC2CFile(ctx, target.OpenID, merge)
	}
	if err != nil {
		return nil, &UploadError{Stage: "merge", UploadID: plan.UploadID, PartIndex: -1, Cause: err}
	}
	if result.FileInfo == "" {
		return nil, &UploadError{Stage: "merge-response", UploadID: plan.UploadID, PartIndex: -1, Cause: errors.New("QQ returned no file_info")}
	}
	return result, nil
}

func (u *Uploader) validatePUTURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return errors.New("invalid signed upload URL")
	}
	if parsed.Scheme != "https" && !(u.config.AllowHTTP && parsed.Scheme == "http") {
		return errors.New("signed upload requires HTTPS")
	}
	return nil
}

func (u *Uploader) uploadChunk(ctx context.Context, target Target, source io.ReaderAt, plan *dto.UploadPrepareResult, item chunk) error {
	checksum := md5.New()
	copied, err := io.Copy(checksum, checkedReader{ctx, io.NewSectionReader(source, item.offset, item.size)})
	if err == nil && copied != item.size {
		err = io.ErrUnexpectedEOF
	}
	if err != nil {
		return &UploadError{Stage: "part-checksum", UploadID: plan.UploadID, PartIndex: item.part.Index, Cause: err}
	}
	if err := u.putChunk(ctx, source, plan.UploadConfig, item); err != nil {
		return &UploadError{Stage: "part-put", UploadID: plan.UploadID, PartIndex: item.part.Index, Cause: err}
	}
	request := &dto.UploadPartFinishRequest{UploadID: plan.UploadID, PartIndex: item.part.Index, BlockSize: dto.DecimalInt64(item.size), MD5: hex.EncodeToString(checksum.Sum(nil))}
	if target.Scope == GroupScope {
		_, err = u.api.FinishGroupUploadPart(ctx, target.OpenID, request)
	} else {
		_, err = u.api.FinishC2CUploadPart(ctx, target.OpenID, request)
	}
	if err != nil {
		return &UploadError{Stage: "part-confirm", UploadID: plan.UploadID, PartIndex: item.part.Index, Cause: err}
	}
	return nil
}

// retrySettings never accelerates retries by truncating a larger server delay.
func (u *Uploader) retrySettings(config dto.UploadConfig) (time.Duration, time.Duration) {
	window := u.config.MaxRetryWindow
	if config.RetryTimeout > 0 && config.RetryTimeout < int(window/time.Second) {
		window = time.Duration(config.RetryTimeout) * time.Second
	}
	delay := time.Second
	if config.RetryDelay > 0 {
		if int64(config.RetryDelay) > (1<<63-1)/int64(time.Second) {
			delay = time.Duration(1<<63 - 1)
		} else {
			delay = time.Duration(config.RetryDelay) * time.Second
		}
	}
	return window, delay
}

func (u *Uploader) putChunk(ctx context.Context, source io.ReaderAt, config dto.UploadConfig, item chunk) error {
	window, delay := u.retrySettings(config)
	retryCtx, stop := context.WithTimeout(ctx, window)
	defer stop()
	for attempt := 0; attempt < u.config.MaxPUTAttempts; attempt++ {
		if err := retryCtx.Err(); err != nil {
			return err
		}
		putCtx, cancel := context.WithTimeout(retryCtx, u.config.PUTTimeout)
		request, err := http.NewRequestWithContext(putCtx, http.MethodPut, item.part.PresignedURL, io.NewSectionReader(source, item.offset, item.size))
		if err != nil {
			cancel()
			return err
		}
		request.ContentLength = item.size
		// Deliberately no Authorization, Cookie, X-Union-Appid or QQ request middleware.
		response, err := u.client.Do(request)
		retryable := err != nil
		if response != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64*1024))
			response.Body.Close()
			if response.StatusCode >= 200 && response.StatusCode < 300 {
				cancel()
				return nil
			}
			retryable = response.StatusCode == 429 || response.StatusCode >= 500
			err = fmt.Errorf("signed PUT returned HTTP %d", response.StatusCode)
		}
		cancel()
		if !retryable || attempt+1 == u.config.MaxPUTAttempts {
			return err
		}
		timer := time.NewTimer(delay)
		select {
		case <-retryCtx.Done():
			timer.Stop()
			return retryCtx.Err()
		case <-timer.C:
		}
	}
	return errors.New("signed PUT retry budget exhausted")
}
