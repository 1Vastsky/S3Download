package job

import (
	"S3Download/internal/downloader"
	"context"
	"sync"
	"time"
)

type Job struct {
	Status downloader.JobStatus
	Cancel context.CancelFunc
}

var (
	store sync.Map // map[string]*Job
)

// New 创建并保存 Job；返回 jobID
func New(j *Job) string {
	id := time.Now().Format("20060102150405.000")
	store.Store(id, j)
	return id
}

func Get(id string) (*Job, bool) {
	v, ok := store.Load(id)
	if !ok {
		return nil, false
	}
	return v.(*Job), true
}

func Delete(id string) { store.Delete(id) }
