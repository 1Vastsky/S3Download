package main

import (
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"S3Download/internal/config"
	"S3Download/internal/downloader"
	"S3Download/internal/handler"
	"S3Download/internal/router"
)

func main() {
	conf := flag.String("c", "config.yaml", "config file")
	addr := flag.String("listen", ":8080", "listen addr")
	flag.Parse()

	cfg, err := config.Load(*conf)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dl, err := downloader.New(cfg)
	if err != nil {
		log.Fatalf("init downloader: %v", err)
	}

	h := handler.New(cfg, dl)
	rt := router.New(h, cfg.Routes)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           rt,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("listen %s", *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(http.ErrServerClosed, err) {
			log.Fatalf("listen: %v", err)
		}
	}()

	// 优雅退出
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("shutting down...")
	_ = srv.Close()
}
