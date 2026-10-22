package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jinline/internal/api"
	"jinline/internal/config"
	"jinline/internal/executor"
	"jinline/internal/queue"
	"jinline/internal/store"

	"gorm.io/gorm"
)

// version 由构建脚本通过 -ldflags "-X main.version=..." 注入。
var version = "dev"

func main() {
	cfgPath := flag.String("c", "configs/config.yaml", "配置文件路径")
	flag.Parse()

	log.Printf("jinline %s starting", version)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	for _, dir := range []string{cfg.Workspace.Root, cfg.Workspace.PipelinesDir, cfg.Workspace.LogsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("创建目录 %s 失败: %v", dir, err)
		}
	}

	db := mustOpenMySQL(cfg.MySQL.DSN)
	q := mustOpenQueue(cfg.Redis.Addr, cfg.Redis.Password, cfg.Redis.DB, cfg.Redis.QueueKey)
	defer q.Close()

	yamls, err := store.NewYamlStore(cfg.Workspace.PipelinesDir)
	if err != nil {
		log.Fatalf("初始化流水线配置存储失败: %v", err)
	}

	exec := executor.New(cfg, db, yamls, q)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	exec.Start(ctx)

	router := api.NewRouter(api.Deps{
		Cfg:     cfg,
		DB:      db,
		Yamls:   yamls,
		Exec:    exec,
		Queue:   q,
		Version: version,
	})

	srv := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("jinline server listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP 服务启动失败: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP 服务关闭异常: %v", err)
	}
}

func mustOpenMySQL(dsn string) *gorm.DB {
	var lastErr error
	for i := 0; i < 30; i++ {
		db, err := store.OpenMySQL(dsn)
		if err == nil {
			return db
		}
		lastErr = err
		log.Printf("等待 MySQL 就绪 (%d/30): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("连接 MySQL 失败: %v", lastErr)
	return nil
}

func mustOpenQueue(addr, password string, db int, key string) *queue.Queue {
	var lastErr error
	for i := 0; i < 15; i++ {
		q, err := queue.New(addr, password, db, key)
		if err == nil {
			return q
		}
		lastErr = err
		log.Printf("等待 Redis 就绪 (%d/15): %v", i+1, err)
		time.Sleep(2 * time.Second)
	}
	log.Fatalf("连接 Redis 失败: %v", lastErr)
	return nil
}
