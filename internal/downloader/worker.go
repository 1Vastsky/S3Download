package downloader

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// JobStatus —— 用于实时汇报任务进度 & 错误信息
// Total     —— 需要下载的文件总数（过滤掉目录条目）
// Finished  —— 已成功下载完成的文件数
// Failed    —— 永久失败（重试耗尽）的文件数
// Running   —— 标记任务是否仍在运行中
// LastError —— 最近一次失败的错误，用于快速排查
type JobStatus struct {
	Total     uint64 `json:"total"`
	Finished  uint64 `json:"finished"`
	Failed    uint64 `json:"failed"`
	Running   bool   `json:"running"`
	LastError string `json:"last_error"`
}

// EnsureDir 保证目标文件所在目录存在；若已存在则立即返回 nil
func EnsureDir(filePath string) error {
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "/" {
		return nil // 当前目录或根目录无需创建
	}
	return os.MkdirAll(dir, 0o755)
}

// Worker 会把多个 prefix（目录）下的对象递归拉取到本地 dest 目录。
// 它遵循以下原则：
//   - 目录对象 (IsDir=true) 直接跳过；
//   - 已存在且大小一致的文件跳过，避免重复下载；
//   - 支持 ctx 取消，方便在 HTTP 层超时 / 用户手动终止；
//   - 指数退避重试 Download，最大次数由 cfg.MaxRetries 决定；
func Worker(ctx context.Context, cli *Client, prefixes []string, dest string, status *JobStatus) {
	status.Running = true
	defer func() { status.Running = false }()

	wg := &sync.WaitGroup{}
	// 控制并发，避免同时打开过多连接 / 占光带宽
	sem := make(chan struct{}, cli.cfg.Concurrency)

	for _, prefix := range prefixes {
		var marker string
		for {
			// --------------------------- 列目录 ---------------------------
			list, next, err := cli.ListObjects(prefix, marker)
			if err != nil {
				status.LastError = err.Error()
				return
			}

			for _, obj := range list {
				if obj.IsDir { // 目录占位对象，跳过
					continue
				}
				atomic.AddUint64(&status.Total, 1)

				// --------------------------- 下载文件 ---------------------------
				wg.Add(1)
				sem <- struct{}{}
				go func(o ObjectInfo) {
					defer func() { <-sem; wg.Done() }()

					// 上层取消时尽快结束 goroutine
					select {
					case <-ctx.Done():
						return
					default:
					}

					key := o.Key
					local := filepath.Join(dest, key)

					// 已存在且大小一致，直接计入结束
					if fi, err := os.Stat(local); err == nil && fi.Size() == o.Size {
						atomic.AddUint64(&status.Finished, 1)
						return
					}

					// 多次重试，指数退避：1s,2s,4s,8s...
					var dlErr error
					for attempt := 0; attempt <= cli.cfg.MaxRetries; attempt++ {
						if attempt > 0 {
							time.Sleep(time.Duration(1<<attempt) * time.Second)
						}

						if err := EnsureDir(local); err != nil {
							dlErr = err
							continue
						}
						dlErr = cli.Download(key, local)
						if dlErr == nil {
							atomic.AddUint64(&status.Finished, 1)
							return
						}
					}

					// 彻底失败
					atomic.AddUint64(&status.Failed, 1)
					status.LastError = dlErr.Error()
				}(obj)
			}

			if next == "" { // 已到达末尾
				break
			}
			marker = next // 继续翻页
		}
	}

	wg.Wait()
}
