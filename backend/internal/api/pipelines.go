package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"jinline/internal/model"
	"jinline/internal/pipeline"
	"jinline/internal/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type handlers struct {
	d Deps
}

type gitView struct {
	URL    string `json:"url"`
	Branch string `json:"branch"`
}

type pipelineSummary struct {
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Git         gitView            `json:"git"`
	Stages      []string           `json:"stages"`
	UpdatedAt   time.Time          `json:"updated_at"`
	LastRun     *model.PipelineRun `json:"last_run"`
}

func (h *handlers) listPipelines(c *gin.Context) {
	defs, err := h.d.Yamls.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 取每个流水线最近一次运行（MVP 数据量小，直接取最近 500 条在内存中分组）
	var runs []model.PipelineRun
	h.d.DB.Order("created_at DESC").Limit(500).Find(&runs)
	latest := make(map[string]*model.PipelineRun, len(runs))
	for i := range runs {
		if _, ok := latest[runs[i].PipelineName]; !ok {
			latest[runs[i].PipelineName] = &runs[i]
		}
	}

	out := make([]pipelineSummary, 0, len(defs))
	for _, def := range defs {
		stageNames := make([]string, 0, len(def.Stages))
		for _, s := range def.Stages {
			stageNames = append(stageNames, s.Name)
		}
		out = append(out, pipelineSummary{
			Name:        def.Name,
			Description: def.Description,
			Git:         gitView{URL: def.Git.URL, Branch: def.Git.Branch},
			Stages:      stageNames,
			UpdatedAt:   h.d.Yamls.ModTime(def.Name),
			LastRun:     latest[def.Name],
		})
	}
	c.JSON(http.StatusOK, out)
}

func (h *handlers) createPipeline(c *gin.Context) {
	content, ok := readYAMLBody(c)
	if !ok {
		return
	}
	def, err := pipeline.Parse([]byte(content))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if h.d.Yamls.Exists(def.Name) {
		c.JSON(http.StatusConflict, gin.H{"error": "同名流水线已存在: " + def.Name})
		return
	}
	if err := h.d.Yamls.SaveRaw([]byte(content), def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, def)
}

func (h *handlers) getPipeline(c *gin.Context) {
	name := c.Param("name")

	if c.Query("raw") != "" {
		content, err := h.d.Yamls.GetRaw(name)
		if err != nil {
			jsonStoreError(c, err)
			return
		}
		c.Data(http.StatusOK, "text/plain; charset=utf-8", content)
		return
	}

	def, err := h.d.Yamls.Get(name)
	if err != nil {
		jsonStoreError(c, err)
		return
	}

	var runs []model.PipelineRun
	h.d.DB.Where("pipeline_name = ?", name).
		Order("created_at DESC").Limit(20).Find(&runs)

	c.JSON(http.StatusOK, gin.H{
		"name":        def.Name,
		"description": def.Description,
		"git":         gitView{URL: def.Git.URL, Branch: def.Git.Branch},
		"workdir":     def.Workdir,
		"env":         def.Env,
		"stages":      def.Stages,
		"runs":        runs,
	})
}

func (h *handlers) updatePipeline(c *gin.Context) {
	oldName := c.Param("name")
	if !pipeline.ValidName(oldName) {
		c.JSON(http.StatusNotFound, gin.H{"error": store.ErrNotFound.Error()})
		return
	}

	content, ok := readYAMLBody(c)
	if !ok {
		return
	}
	def, err := pipeline.Parse([]byte(content))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !h.d.Yamls.Exists(oldName) {
		c.JSON(http.StatusNotFound, gin.H{"error": store.ErrNotFound.Error()})
		return
	}
	if def.Name != oldName && h.d.Yamls.Exists(def.Name) {
		c.JSON(http.StatusConflict, gin.H{"error": "同名流水线已存在: " + def.Name})
		return
	}
	if err := h.d.Yamls.SaveRaw([]byte(content), def); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if def.Name != oldName {
		if err := h.d.Yamls.Delete(oldName); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}
	c.JSON(http.StatusOK, def)
}

func (h *handlers) deletePipeline(c *gin.Context) {
	name := c.Param("name")
	if err := h.d.Yamls.Delete(name); err != nil {
		jsonStoreError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (h *handlers) triggerRun(c *gin.Context) {
	name := c.Param("name")
	def, err := h.d.Yamls.Get(name)
	if err != nil {
		jsonStoreError(c, err)
		return
	}

	var body struct {
		Branch string `json:"branch"`
	}
	_ = c.ShouldBindJSON(&body) // 允许空请求体

	branch := body.Branch
	if branch == "" {
		branch = def.Git.Branch
	}

	run := &model.PipelineRun{
		ID:           uuid.NewString(),
		PipelineName: def.Name,
		Branch:       branch,
		Status:       model.StatusPending,
		Trigger:      "manual",
	}
	if err := h.d.DB.Create(run).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.d.Queue.Enqueue(c.Request.Context(), run.ID); err != nil {
		now := time.Now()
		h.d.DB.Model(run).Updates(map[string]any{
			"status":      model.StatusFailed,
			"error":       "任务入队失败: " + err.Error(),
			"finished_at": &now,
		})
		c.JSON(http.StatusInternalServerError, gin.H{"error": "任务入队失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, run)
}

func readYAMLBody(c *gin.Context) (string, bool) {
	if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
		var body struct {
			YAML string `json:"yaml"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求体格式错误: " + err.Error()})
			return "", false
		}
		return body.YAML, true
	}

	data, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return "", false
	}
	return string(data), true
}

func jsonStoreError(c *gin.Context, err error) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
