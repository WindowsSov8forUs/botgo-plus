// Package media uploads native QQ media without converting messages or sending them automatically.
package media

import (
	"context"
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"github.com/WindowsSov8forUs/botgo-plus/dto"
	"github.com/WindowsSov8forUs/botgo-plus/log"
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
	Stage       string
	UploadID    string
	PartIndex   int
	Cause       error
	Meta        *v1.ResponseMeta // The failed phase response, kept in memory and never dumped by Error.
	PlanSummary string
}

func (e *UploadError) Error() string {
	if e == nil {
		return ""
	}
	text := "QQ media upload failed at " + e.Stage
	if e.PartIndex >= 0 {
		text += fmt.Sprintf(" (part %d)", e.PartIndex)
	}
	if e.PlanSummary != "" {
		text += ": " + e.PlanSummary
	}
	if e.Cause != nil {
		text += ": " + log.SafeError(e.Cause)
	}
	return text
}
func (e *UploadError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
func (e *UploadError) ResponseHeaders() http.Header {
	if e == nil {
		return nil
	}
	if e.Meta != nil {
		h := e.Meta.Header.Clone()
		if h == nil {
			h = make(http.Header)
		}
		if e.Meta.TraceID != "" {
			h.Set("X-Tps-trace-ID", e.Meta.TraceID)
		}
		return h
	}
	var response interface{ ResponseHeaders() http.Header }
	if errors.As(e.Cause, &response) {
		return response.ResponseHeaders()
	}
	return nil
}

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
	if plan == nil {
		return nil, errors.New("no upload plan was returned")
	}
	if plan.UploadID == "" {
		return nil, errors.New("upload plan has no upload_id")
	}
	if len(plan.Parts) == 0 {
		return nil, errors.New("upload plan has no parts")
	}
	if len(plan.Parts) > 65536 {
		return nil, fmt.Errorf("upload plan contains too many parts: %d", len(plan.Parts))
	}
	parts := append([]dto.UploadPart(nil), plan.Parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].Index < parts[j].Index })
	result := make([]chunk, 0, len(parts))
	var offset int64
	for index, part := range parts {
		if part.Index != index {
			return nil, fmt.Errorf("non-contiguous upload parts: expected index %d, received %d", index, part.Index)
		}
		if part.PresignedURL == "" {
			return nil, fmt.Errorf("upload part %d has no signed URL", part.Index)
		}
		if offset >= size {
			return nil, fmt.Errorf("upload part %d exceeds source size %d", part.Index, size)
		}
		length := int64(part.BlockSize)
		if length == 0 {
			length = int64(plan.BlockSize)
		}
		if length <= 0 {
			return nil, fmt.Errorf("invalid upload block size %d for part %d", length, part.Index)
		}
		if remaining := size - offset; length > remaining {
			length = remaining
		}
		result = append(result, chunk{part: part, offset: offset, size: length})
		offset += length
	}
	if offset != size {
		return nil, fmt.Errorf("upload plan covers %d of %d source bytes", offset, size)
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
	var meta *v1.ResponseMeta
	if target.Scope == GroupScope {
		plan, meta, err = u.api.PrepareGroupUpload(ctx, target.OpenID, request)
	} else {
		plan, meta, err = u.api.PrepareC2CUpload(ctx, target.OpenID, request)
	}
	if err != nil {
		return nil, &UploadError{Stage: "prepare", PartIndex: -1, Cause: err, Meta: meta}
	}
	uploadID := ""
	if plan != nil {
		uploadID = plan.UploadID
	}
	chunks, err := uploadLayout(plan, size)
	if err != nil {
		return nil, &UploadError{Stage: "layout", UploadID: uploadID, PartIndex: -1, Cause: meta.WrapError("validate QQ upload plan", err), Meta: meta, PlanSummary: describeUploadPlan(plan, meta)}
	}
	for _, item := range chunks {
		if err := u.validatePUTURL(item.part.PresignedURL); err != nil {
			return nil, &UploadError{Stage: "signed-url", UploadID: plan.UploadID, PartIndex: item.part.Index, Cause: meta.WrapError("validate QQ upload target", err), Meta: meta}
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
		result, meta, err = u.api.UploadGroupFile(ctx, target.OpenID, merge)
	} else {
		result, meta, err = u.api.UploadC2CFile(ctx, target.OpenID, merge)
	}
	if err != nil {
		return nil, &UploadError{Stage: "merge", UploadID: plan.UploadID, PartIndex: -1, Cause: err, Meta: meta}
	}
	if result == nil || result.FileInfo == "" {
		return nil, &UploadError{Stage: "merge-response", UploadID: plan.UploadID, PartIndex: -1, Cause: meta.WrapError("validate merged QQ file", errors.New("QQ returned no file_info")), Meta: meta}
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
	var meta *v1.ResponseMeta
	if target.Scope == GroupScope {
		meta, err = u.api.FinishGroupUploadPart(ctx, target.OpenID, request)
	} else {
		meta, err = u.api.FinishC2CUploadPart(ctx, target.OpenID, request)
	}
	if err != nil {
		return &UploadError{Stage: "part-confirm", UploadID: plan.UploadID, PartIndex: item.part.Index, Cause: err, Meta: meta}
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
	var last error
	for attempt := 0; attempt < u.config.MaxPUTAttempts; attempt++ {
		if err := retryCtx.Err(); err != nil {
			return errors.Join(err, last)
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
		wait := delay
		if response != nil {
			data, readErr := io.ReadAll(io.LimitReader(response.Body, 64*1024+1))
			response.Body.Close()
			if err == nil && readErr == nil && response.StatusCode >= 200 && response.StatusCode < 300 {
				cancel()
				return nil
			}
			retryable = readErr != nil || response.StatusCode == 429 || response.StatusCode >= 500
			last = putResponseError(response, data, errors.Join(err, readErr))
			if hint := retryAfter(response.Header); hint > wait {
				wait = hint
			}
		} else {
			last = err
		}
		cancel()
		if !retryable || attempt+1 == u.config.MaxPUTAttempts {
			return last
		}
		if err := waitUploadRetry(retryCtx, wait); err != nil {
			return errors.Join(err, last)
		}
	}
	return last
}

func describeUploadPlan(plan *dto.UploadPrepareResult, meta *v1.ResponseMeta) string {
	if plan == nil {
		return "no upload plan was returned"
	}
	var fields map[string]json.RawMessage
	if meta != nil {
		_ = json.Unmarshal(meta.Raw, &fields)
	}
	_, fileInfo := fields["file_info"]
	_, fileUUID := fields["file_uuid"]
	return fmt.Sprintf("upload ID present: %t; block size: %d; parts: %d; file_info present: %t; file_uuid present: %t", plan.UploadID != "", plan.BlockSize, len(plan.Parts), fileInfo, fileUUID)
}
