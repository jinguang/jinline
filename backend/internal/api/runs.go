package api

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"jinline/internal/model"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var safeIDRe = regexp.MustCompile(`^[a-zA-Z0-9-]{1,64}$`)

func (h *handlers) listRuns(c *gin.Context) {
	page := atoiDefault(c.Query("page"), 1)
	if page < 1 {
		page = 1
	}
	pageSize := atoiDefault(c.Query("page_size"), 20)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := h.d.DB.Model(&model.PipelineRun{})
	if name := c.Query("pipeline"); name != "" {
		q = q.Where("pipeline_name = ?", name)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var runs []model.PipelineRun
	if err := q.Order("created_at DESC").
		Offset((page - 1) * pageSize).Limit(pageSize).
		Find(&runs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"total": total, "items": runs})
}

func (h *handlers) getRun(c *gin.Context) {
	id := c.Param("id")
	if !safeIDRe.MatchString(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	run := &model.PipelineRun{}
	err := h.d.DB.
		Preload("Stages", func(db *gorm.DB) *gorm.DB { return db.Order("id ASC") }).
		Where("id = ?", id).First(run).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "运行记录不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, run)
}

// runLogs 以增量方式返回运行日志：客户端携带已读取的 offset，
// 服务端读取日志文件中 offset 之后的内容，并通过响应头返回新 offset 与运行状态。
func (h *handlers) runLogs(c *gin.Context) {
	id := c.Param("id")
	if !safeIDRe.MatchString(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	run := &model.PipelineRun{}
	if err := h.d.DB.Where("id = ?", id).First(run).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运行记录不存在"})
		return
	}

	offset := int64(atoiDefault(c.Query("offset"), 0))
	if offset < 0 {
		offset = 0
	}

	logPath := filepath.Join(h.d.Cfg.Workspace.LogsDir, id+".log")
	var data []byte
	if f, err := os.Open(logPath); err == nil {
		if offset > 0 {
			_, _ = f.Seek(offset, io.SeekStart)
		}
		data, _ = io.ReadAll(f)
		_ = f.Close()
	} else if !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Header("X-Log-Offset", strconv.FormatInt(offset+int64(len(data)), 10))
	c.Header("X-Run-Status", run.Status)
	c.Data(http.StatusOK, "text/plain; charset=utf-8", data)
}

func (h *handlers) cancelRun(c *gin.Context) {
	id := c.Param("id")
	if !safeIDRe.MatchString(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}

	run := &model.PipelineRun{}
	if err := h.d.DB.Where("id = ?", id).First(run).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "运行记录不存在"})
		return
	}

	switch run.Status {
	case model.StatusPending:
		now := time.Now()
		h.d.DB.Model(run).Updates(map[string]any{
			"status":      model.StatusCanceled,
			"error":       "手动取消",
			"finished_at": &now,
		})
		// 若 worker 恰好已取到任务，尝试一并取消
		h.d.Exec.Cancel(id)
		c.JSON(http.StatusOK, gin.H{"status": model.StatusCanceled})

	case model.StatusRunning:
		if h.d.Exec.Cancel(id) {
			c.JSON(http.StatusOK, gin.H{"status": "canceling"})
			return
		}
		// 执行器可能刚好结束，返回最新状态
		_ = h.d.DB.Where("id = ?", id).First(run).Error
		c.JSON(http.StatusOK, gin.H{"status": run.Status})

	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "运行已结束: " + run.Status})
	}
}

func atoiDefault(s string, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}
