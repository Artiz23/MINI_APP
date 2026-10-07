package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"tg-bot-orh3/MINI_APP/internal/appdb"
	"tg-bot-orh3/MINI_APP/internal/hub"
	"tg-bot-orh3/MINI_APP/internal/secrets"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	root := findRoot()
	dataPath := filepath.Join(root, "data", "miniapp.json")
	webDir := filepath.Join(root, "web")

	db, err := appdb.Load(dataPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}

	bot, err := hub.NewBot(db)
	if err != nil {
		log.Fatalf("bot: %v", err)
	}

	api := hub.NewAPI(db, bot, webDir)
	srv := &http.Server{
		Addr:              secrets.ListenAddr,
		Handler:           api.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cr := hub.StartCron(api)
	defer cr.Stop()

	go func() {
		log.Printf("miniapp http %s  web=%s", secrets.ListenAddr, webDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()
	go bot.Listen(ctx)

	<-ctx.Done()
	shut, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_ = srv.Shutdown(shut)
}

func findRoot() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		if exist(filepath.Join(dir, "web", "index.html")) {
			return dir
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if exist(filepath.Join(wd, "web", "index.html")) {
			return wd
		}
		if exist(filepath.Join(wd, "MINI_APP", "web", "index.html")) {
			return filepath.Join(wd, "MINI_APP")
		}
	}
	return "."
}

func exist(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
