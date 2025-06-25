package downloader

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"S3Download/internal/config"
)

type ObjectInfo struct {
	Key   string `json:"key"`
	Size  int64  `json:"size,omitempty"`
	Mtime string `json:"mtime,omitempty"`
	IsDir bool   `json:"is_dir"`
}

type Client struct {
	cli *minio.Client
	cfg *config.Config
}

func New(cfg *config.Config) (*Client, error) {
	useSSL := strings.HasPrefix(cfg.Endpoint, "https")
	mc, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.AccessKeySecret, ""),
		Secure: useSSL,
		Region: "", // MinIO 不要求 region；保留字段兼容 S3
	})
	if err != nil {
		return nil, err
	}
	return &Client{cli: mc, cfg: cfg}, nil
}

// Download Download：使用 FGetObject 自带多线程下载，支持断点续传
func (c *Client) Download(key, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()
	return c.cli.FGetObject(ctx, c.cfg.Bucket, key, dest, minio.GetObjectOptions{})
}

// ListObjects ListObjects：一次最多列 1000 条，仿 OSS “子目录” 语义
func (c *Client) ListObjects(prefix, marker string) ([]ObjectInfo, string, error) {
	const limit = 1000
	opts := minio.ListObjectsOptions{
		Prefix:     prefix,
		Recursive:  false,  // 配合斜杠语义
		StartAfter: marker, // 分页
		MaxKeys:    limit,
	}

	ctx := context.Background()
	out := make([]ObjectInfo, 0, limit)
	var lastKey string

	for obj := range c.cli.ListObjects(ctx, c.cfg.Bucket, opts) {
		if obj.Err != nil {
			return nil, "", obj.Err
		}
		lastKey = obj.Key
		isDir := strings.HasSuffix(obj.Key, "/")
		out = append(out, ObjectInfo{
			Key:   obj.Key,
			Size:  obj.Size,
			Mtime: obj.LastModified.Format(time.RFC3339),
			IsDir: isDir,
		})
	}
	if lastKey == "" { // 空结果
		return out, "", nil
	}
	// 判断是否还有下一页
	if len(out) < limit {
		return out, "", nil
	}
	return out, lastKey, nil
}
