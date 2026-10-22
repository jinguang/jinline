package model

import "time"

const (
	StatusPending  = "pending"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusCanceled = "canceled"
)

// PipelineRun 记录一次流水线运行的状态信息，持久化到 MySQL。
type PipelineRun struct {
	ID           string     `gorm:"primaryKey;size:40" json:"id"`
	PipelineName string     `gorm:"size:128;index" json:"pipeline_name"`
	Branch       string     `gorm:"size:128" json:"branch"`
	Commit       string     `gorm:"size:64" json:"commit"`
	Message      string     `gorm:"size:512" json:"message"`
	Status       string     `gorm:"size:16;index" json:"status"`
	Trigger      string     `gorm:"size:16" json:"trigger"`
	Error        string     `gorm:"size:1024" json:"error"`
	StartedAt    *time.Time `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at"`
	CreatedAt    time.Time  `json:"created_at"`
	Stages       []RunStage `gorm:"foreignKey:RunID;constraint:OnDelete:CASCADE" json:"stages,omitempty"`
}

// RunStage 记录运行中每个阶段的执行状态。
type RunStage struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	RunID      string     `gorm:"size:40;index" json:"run_id"`
	Name       string     `gorm:"size:128" json:"name"`
	Type       string     `gorm:"size:16" json:"type"`
	Status     string     `gorm:"size:16" json:"status"`
	ExitCode   int        `json:"exit_code"`
	DurationMs int64      `json:"duration_ms"`
	StartedAt  *time.Time `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at"`
}
