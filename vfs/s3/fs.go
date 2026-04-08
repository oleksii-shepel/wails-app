// Package s3 provides a read-only vfs.FileSystem backed by an S3-compatible
// object store such as AWS S3 or MinIO.
package s3

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"wails-app/vfs"
)

const readChunkSize = 8 * 1024 * 1024

// Config holds S3-compatible connection parameters.
type Config struct {
	Region          string
	Bucket          string
	Prefix          string
	Endpoint        string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	UsePathStyle    bool
	Insecure        bool
}

// FS implements vfs.FileSystem over one bucket and optional prefix.
type FS struct {
	client *s3sdk.Client
	bucket string
	prefix string
}

var _ vfs.FileSystem = (*FS)(nil)

// New creates an S3-backed filesystem rooted at cfg.Bucket/cfg.Prefix.
func New(ctx context.Context, cfg Config) (*FS, error) {
	bucket := strings.TrimSpace(cfg.Bucket)
	if bucket == "" {
		return nil, fmt.Errorf("s3: bucket is required")
	}

	region := strings.TrimSpace(cfg.Region)
	if region == "" {
		region = "us-east-1"
	}

	loadOptions := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(region),
	}

	if cfg.AccessKeyID != "" || cfg.SecretAccessKey != "" || cfg.SessionToken != "" {
		loadOptions = append(loadOptions,
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
				cfg.AccessKeyID,
				cfg.SecretAccessKey,
				cfg.SessionToken,
			)),
		)
	}

	if endpoint := normalizeEndpoint(cfg.Endpoint, cfg.Insecure); endpoint != "" {
		loadOptions = append(loadOptions, awsconfig.WithBaseEndpoint(endpoint))
	}

	if cfg.Insecure {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
		loadOptions = append(loadOptions, awsconfig.WithHTTPClient(&http.Client{Transport: transport}))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("s3: load config: %w", err)
	}

	client := s3sdk.NewFromConfig(awsCfg, func(o *s3sdk.Options) {
		o.UsePathStyle = cfg.UsePathStyle
	})

	if _, err := client.HeadBucket(ctx, &s3sdk.HeadBucketInput{Bucket: aws.String(bucket)}); err != nil {
		return nil, fmt.Errorf("s3: head bucket %s: %w", bucket, mapErr(err))
	}

	return &FS{
		client: client,
		bucket: bucket,
		prefix: cleanPrefix(cfg.Prefix),
	}, nil
}

// Close releases idle HTTP connections held by the AWS client.
func (f *FS) Close() error {
	if transport, ok := f.client.Options().HTTPClient.(*http.Client); ok {
		if httpTransport, ok := transport.Transport.(*http.Transport); ok {
			httpTransport.CloseIdleConnections()
		}
	}
	return nil
}

func (f *FS) Open(ctx context.Context, p string) (vfs.File, error) {
	cleaned := cleanPath(p)
	if cleaned == "" {
		return nil, vfs.ErrNotFile
	}

	info, err := f.statObject(ctx, cleaned)
	if err != nil {
		return nil, err
	}

	return &readFile{
		ctx:    ctx,
		client: f.client,
		bucket: f.bucket,
		key:    f.objectKey(cleaned),
		size:   info.size,
		info:   info,
	}, nil
}

func (f *FS) Stat(ctx context.Context, p string) (vfs.FileInfo, error) {
	cleaned := cleanPath(p)
	if cleaned == "" {
		return &fileInfo{name: f.rootName(), isDir: true}, nil
	}

	if info, err := f.statObject(ctx, cleaned); err == nil {
		return info, nil
	} else if !errors.Is(err, vfs.ErrNotFound) {
		return nil, err
	}

	exists, err := f.directoryExists(ctx, cleaned)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, vfs.ErrNotFound
	}

	return &fileInfo{name: path.Base(cleaned), isDir: true}, nil
}

// ReadDir lists one pseudo-directory level using common prefixes and direct objects.
func (f *FS) ReadDir(ctx context.Context, p string) ([]vfs.FileInfo, error) {
	cleaned := cleanPath(p)
	if cleaned != "" {
		info, err := f.Stat(ctx, cleaned)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, vfs.ErrNotDir
		}
	}

	prefix := f.directoryKey(cleaned)
	input := &s3sdk.ListObjectsV2Input{
		Bucket:    aws.String(f.bucket),
		Delimiter: aws.String("/"),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix)
	}

	paginator := s3sdk.NewListObjectsV2Paginator(f.client, input)
	entries := make([]vfs.FileInfo, 0)
	seen := make(map[string]struct{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, mapErr(err)
		}

		for _, commonPrefix := range page.CommonPrefixes {
			name := childName(prefix, aws.ToString(commonPrefix.Prefix))
			if name == "" {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			entries = append(entries, &fileInfo{name: name, isDir: true})
		}

		for _, object := range page.Contents {
			key := aws.ToString(object.Key)
			name := childName(prefix, key)
			if name == "" || strings.Contains(name, "/") {
				continue
			}
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			entries = append(entries, &fileInfo{
				name:    name,
				size:    aws.ToInt64(object.Size),
				modTime: timeValue(object.LastModified),
				isDir:   false,
			})
		}
	}

	return entries, nil
}

func (f *FS) Exists(ctx context.Context, p string) bool {
	_, err := f.Stat(ctx, p)
	return err == nil
}

func (f *FS) Create(_ context.Context, _ string) (vfs.File, error) {
	return nil, vfs.ErrReadOnly
}

func (f *FS) Mkdir(_ context.Context, _ string, _ fs.FileMode) error {
	return vfs.ErrReadOnly
}

func (f *FS) Remove(_ context.Context, _ string) error {
	return vfs.ErrReadOnly
}

func (f *FS) Rename(_ context.Context, _, _ string) error {
	return vfs.ErrReadOnly
}

func (f *FS) Join(elem ...string) string      { return path.Join(elem...) }
func (f *FS) Split(p string) (string, string) { return path.Split(p) }
func (f *FS) IsAbs(p string) bool             { return strings.HasPrefix(p, "/") }
func (f *FS) RealPath(_ string) (string, error) {
	return "", nil
}

func (f *FS) statObject(ctx context.Context, p string) (*fileInfo, error) {
	output, err := f.client.HeadObject(ctx, &s3sdk.HeadObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(f.objectKey(p)),
	})
	if err != nil {
		return nil, mapErr(err)
	}

	return &fileInfo{
		name:    path.Base(p),
		size:    aws.ToInt64(output.ContentLength),
		modTime: timeValue(output.LastModified),
		isDir:   false,
	}, nil
}

func (f *FS) directoryExists(ctx context.Context, p string) (bool, error) {
	prefix := f.directoryKey(p)
	if prefix == "" {
		return true, nil
	}

	page, err := f.client.ListObjectsV2(ctx, &s3sdk.ListObjectsV2Input{
		Bucket:    aws.String(f.bucket),
		Prefix:    aws.String(prefix),
		Delimiter: aws.String("/"),
		MaxKeys:   aws.Int32(1),
	})
	if err != nil {
		return false, mapErr(err)
	}

	return len(page.CommonPrefixes) > 0 || len(page.Contents) > 0, nil
}

func (f *FS) rootName() string {
	if f.prefix != "" {
		return path.Base(f.prefix)
	}
	return f.bucket
}

func (f *FS) objectKey(p string) string {
	cleaned := cleanPath(p)
	if f.prefix == "" {
		return cleaned
	}
	if cleaned == "" {
		return f.prefix
	}
	return f.prefix + "/" + cleaned
}

func (f *FS) directoryKey(p string) string {
	combined := f.objectKey(p)
	if combined == "" {
		return ""
	}
	return strings.TrimSuffix(combined, "/") + "/"
}

type fileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (fi *fileInfo) Name() string { return fi.name }
func (fi *fileInfo) Size() int64  { return fi.size }
func (fi *fileInfo) Mode() fs.FileMode {
	if fi.isDir {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (fi *fileInfo) ModTime() time.Time   { return fi.modTime }
func (fi *fileInfo) IsDir() bool          { return fi.isDir }
func (fi *fileInfo) Sys() any             { return nil }
func (fi *fileInfo) Path() string         { return fi.name }
func (fi *fileInfo) RealPath() string     { return "" }
func (fi *fileInfo) Volume() vfs.Volume   { return nil }
func (fi *fileInfo) IsArchive() bool      { return !fi.isDir && isArchiveExt(fi.name) }
func (fi *fileInfo) IsHidden() bool       { return strings.HasPrefix(fi.name, ".") }
func (fi *fileInfo) IsSymlink() bool      { return false }
func (fi *fileInfo) IsNested() bool       { return false }
func (fi *fileInfo) MountChain() []string { return nil }

var _ vfs.FileInfo = (*fileInfo)(nil)

type readFile struct {
	ctx       context.Context
	client    *s3sdk.Client
	bucket    string
	key       string
	size      int64
	position  int64
	body      io.ReadCloser
	bodyStart int64
	bodyEnd   int64
	info      *fileInfo
}

func (f *readFile) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if f.position >= f.size {
		return 0, io.EOF
	}
	if err := f.ensureBody(); err != nil {
		return 0, err
	}

	n, err := f.body.Read(p)
	f.position += int64(n)
	if err == io.EOF {
		_ = f.closeBody()
		if f.position < f.size && n > 0 {
			return n, nil
		}
	}
	return n, err
}

func (f *readFile) Write(_ []byte) (int, error) {
	return 0, vfs.ErrReadOnly
}

func (f *readFile) Seek(offset int64, whence int) (int64, error) {
	var base int64
	switch whence {
	case io.SeekStart:
		base = 0
	case io.SeekCurrent:
		base = f.position
	case io.SeekEnd:
		base = f.size
	default:
		return 0, fmt.Errorf("s3: invalid seek whence %d", whence)
	}

	next := base + offset
	if next < 0 {
		return 0, fmt.Errorf("s3: negative seek position")
	}
	f.position = next
	_ = f.closeBody()
	return next, nil
}

func (f *readFile) Close() error {
	return f.closeBody()
}

func (f *readFile) Stat() (vfs.FileInfo, error) {
	return f.info, nil
}

func (f *readFile) Sync() error { return nil }

var _ vfs.File = (*readFile)(nil)

func (f *readFile) ensureBody() error {
	if f.body != nil && f.position >= f.bodyStart && f.position <= f.bodyEnd {
		return nil
	}
	_ = f.closeBody()

	end := minInt64(f.size-1, f.position+readChunkSize-1)
	output, err := f.client.GetObject(f.ctx, &s3sdk.GetObjectInput{
		Bucket: aws.String(f.bucket),
		Key:    aws.String(f.key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", f.position, end)),
	})
	if err != nil {
		return mapErr(err)
	}

	f.body = output.Body
	f.bodyStart = f.position
	f.bodyEnd = end
	return nil
}

func (f *readFile) closeBody() error {
	if f.body == nil {
		return nil
	}
	err := f.body.Close()
	f.body = nil
	return err
}

func cleanPrefix(prefix string) string {
	cleaned := cleanPath(prefix)
	if cleaned == "." {
		return ""
	}
	return cleaned
}

func cleanPath(p string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
	trimmed = strings.Trim(trimmed, "/")
	if trimmed == "" || trimmed == "." {
		return ""
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func childName(basePrefix, key string) string {
	if key == "" {
		return ""
	}
	relative := strings.TrimPrefix(key, basePrefix)
	relative = strings.Trim(relative, "/")
	if relative == "" {
		return ""
	}
	parts := strings.Split(relative, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func normalizeEndpoint(endpoint string, insecure bool) string {
	trimmed := strings.TrimSpace(endpoint)
	if trimmed == "" {
		return ""
	}
	if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
		return trimmed
	}
	if insecure {
		return "http://" + trimmed
	}
	return "https://" + trimmed
}

func timeValue(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return *value
}

func minInt64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func isArchiveExt(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".zip", ".tar", ".tar.gz", ".tgz", ".tar.bz2", ".tar.xz", ".rar", ".7z"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

func mapErr(err error) error {
	if err == nil {
		return nil
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchBucket", "NoSuchKey", "NoSuchUpload":
			return vfs.ErrNotFound
		}
	}

	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "not found") || strings.Contains(lower, "no such key") || strings.Contains(lower, "status code: 404") {
		return vfs.ErrNotFound
	}
	if strings.Contains(lower, "forbidden") || strings.Contains(lower, "access denied") {
		return vfs.ErrReadOnly
	}
	return err
}
