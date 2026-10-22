package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"jinline/internal/config"
	"jinline/internal/executor"
	"jinline/internal/queue"
	"jinline/internal/store"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Deps struct {
	Cfg     *config.Config
	DB      *gorm.DB
	Yamls   *store.YamlStore
	Exec    *executor.Executor
	Queue   *queue.Queue
	Version string
}

func NewRouter(d Deps) *gin.Engine {
	if os.Getenv("GIN_MODE") == "" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), cors())
	r.MaxMultipartMemory = 8 << 20

	h := &handlers{d: d}

	api := r.Group("/api")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "version": d.Version})
		})

		api.GET("/pipelines", h.listPipelines)
		api.POST("/pipelines", h.createPipeline)
		api.GET("/pipelines/:name", h.getPipeline)
		api.PUT("/pipelines/:name", h.updatePipeline)
		api.DELETE("/pipelines/:name", h.deletePipeline)
		api.POST("/pipelines/:name/run", h.triggerRun)

		api.GET("/runs", h.listRuns)
		api.GET("/runs/:id", h.getRun)
		api.GET("/runs/:id/logs", h.runLogs)
		api.POST("/runs/:id/cancel", h.cancelRun)
	}

	serveFrontend(r, d.Cfg.Server.StaticDir)
	return r
}

func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		}
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
		c.Header("Access-Control-Expose-Headers", "X-Log-Offset,X-Run-Status")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// serveFrontend 在前端构建产物存在时直接托管 SPA，非静态资源回退到 index.html。
func serveFrontend(r *gin.Engine, dir string) {
	if dir == "" {
		return
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return
	}

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if strings.HasPrefix(p, "/api") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		rel := filepath.Clean(strings.TrimPrefix(filepath.FromSlash(p), string(filepath.Separator)))
		if rel == "." {
			c.File(filepath.Join(dir, "index.html"))
			return
		}
		if strings.HasPrefix(rel, "..") {
			c.Status(http.StatusNotFound)
			return
		}

		full := filepath.Join(dir, rel)
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			c.File(full)
			return
		}
		c.File(filepath.Join(dir, "index.html"))
	})
}
