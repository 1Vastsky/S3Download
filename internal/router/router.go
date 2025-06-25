package router

import (
	"S3Download/internal/config"
	"net/http"

	"github.com/go-chi/chi/v5"

	"S3Download/internal/handler"
)

func New(h *handler.Handler, routes config.Routes) http.Handler {
	r := chi.NewRouter()

	//r.Get("/healthz", h.Healthz)
	//r.Post(routes.Start, h.Start)
	//r.Get(routes.Status, h.Status)
	r.Delete(routes.Status, h.Cancel)
	r.Get(routes.List, h.List)

	return r
}
