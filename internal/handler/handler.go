package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"S3Download/internal/config"
	"S3Download/internal/downloader"
	"S3Download/internal/job"
)

// Handler 聚合依赖，用于 http 层路由回调。
// main.go 示例：
//
//	h := handler.New(cfg, dl)
//	r.Post("/download", h.Start)
//	r.Get("/download/{id}", h.Status)
//	r.Delete("/download/{id}", h.Cancel)
//	r.Get("/objects", h.List)
//	r.Get("/healthz", h.Healthz)
type Handler struct {
	cfg *config.Config
	dl  *downloader.Client
}

func New(cfg *config.Config, dl *downloader.Client) *Handler {
	return &Handler{cfg: cfg, dl: dl}
}

// 通用 JSON 响应封装
type errResp struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errResp{Error: msg})
}

// DTO
type startReq struct {
	Prefixes []string `json:"prefixes"`
	Bucket   string   `json:"bucket,omitempty"` // 预留：动态切 bucket
	Dest     string   `json:"dest,omitempty"`
}

type startResp struct {
	JobID string `json:"job_id"`
}

type listResp struct {
	Objects    []downloader.ObjectInfo `json:"objects"`
	NextMarker string                  `json:"next_marker"`
	Truncated  bool                    `json:"truncated"`
}

const maxPageSize = 1000

// Start POST /download —— 创建下载任务
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	var req startReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if len(req.Prefixes) == 0 {
		writeError(w, http.StatusBadRequest, "prefixes must not be empty")
		return
	}

	dest := h.cfg.Dest
	if req.Dest != "" {
		dest = req.Dest
	}

	ctx, cancel := context.WithCancel(r.Context())
	j := &job.Job{Cancel: cancel}
	id := job.New(j)

	go downloader.Worker(ctx, h.dl, req.Prefixes, dest, &j.Status)

	writeJSON(w, http.StatusAccepted, startResp{JobID: id})
}

// Status GET /download/{id} —— 查询任务状态
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j, ok := job.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, j.Status)
}

// Cancel DELETE /download/{id} —— 取消任务
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	j, ok := job.Get(id)
	if !ok {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	j.Cancel()
	job.Delete(id)
	w.WriteHeader(http.StatusNoContent)
}

// List GET /objects —— 分页列目录
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	prefix := q.Get("prefix")
	marker := q.Get("marker")

	limit := maxPageSize
	if limStr := q.Get("limit"); limStr != "" {
		n, err := strconv.Atoi(limStr)
		if err != nil || n <= 0 || n > maxPageSize {
			writeError(w, http.StatusBadRequest, "limit must be 1-1000")
			return
		}
		limit = n
	}

	// downloader.Client 目前仅暴露 ListObjects(prefix,marker) (<=1000 条/页)
	objs, next, err := h.dl.ListObjects(prefix, marker)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 根据 limit 截取结果
	truncated := false
	if len(objs) > limit {
		objs = objs[:limit]
		truncated = true
		next = objs[limit-1].Key // 下一页从当前最后一条开始
	} else if next != "" {
		truncated = true // 后端还有更多
	}

	writeJSON(w, http.StatusOK, listResp{
		Objects:    objs,
		NextMarker: next,
		Truncated:  truncated,
	})
}

// Healthz GET /healthz —— 探活
func (h *Handler) Healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
