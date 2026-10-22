package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"jinline/internal/model"
	"jinline/internal/pipeline"
)

// Execute 执行一次流水线运行。整体流程：
// 检出代码 -> 依次执行各阶段 -> 更新 MySQL 状态，日志写入本地文件。
func (e *Executor) Execute(ctx context.Context, runID string) {
	run := &model.PipelineRun{}
	if err := e.db.Where("id = ?", runID).First(run).Error; err != nil {
		log.Printf("load run %s: %v", runID, err)
		return
	}
	// 排队期间被取消的运行直接跳过
	if run.Status != model.StatusPending {
		return
	}

	runCtx, cancel := context.WithCancel(ctx)
	e.registerCancel(runID, cancel)
	defer e.unregisterCancel(runID)

	now := time.Now()
	run.Status = model.StatusRunning
	run.StartedAt = &now
	e.saveRun(run)

	logPath := filepath.Join(e.cfg.Workspace.LogsDir, runID+".log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		e.finishRun(run, model.StatusFailed, "打开日志文件失败: "+err.Error())
		return
	}
	defer logFile.Close()
	lg := &fileLogger{f: logFile}

	lg.printf("==> 开始运行 %s（流水线: %s）", runID, run.PipelineName)

	def, err := e.yamls.Get(run.PipelineName)
	if err != nil {
		e.fail(run, lg, "加载流水线配置失败: "+err.Error())
		return
	}

	runDir := filepath.Join(e.cfg.Workspace.Root, runID)
	srcDir := filepath.Join(runDir, "src")
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		e.fail(run, lg, "创建构建目录失败: "+err.Error())
		return
	}

	// ---- checkout ----
	checkoutStage := e.startStage(runID, "checkout", "git")
	secrets := []string{def.Git.Password}
	err = e.checkout(runCtx, run, def, srcDir, secrets, lg)
	if err != nil {
		if runCtx.Err() != nil {
			e.endStage(checkoutStage, model.StatusCanceled, -1)
			lg.printf("==> 运行已取消")
			e.finishRun(run, model.StatusCanceled, "手动取消")
			return
		}
		e.endStage(checkoutStage, model.StatusFailed, -1)
		e.fail(run, lg, "checkout 失败: "+err.Error())
		return
	}
	e.endStage(checkoutStage, model.StatusSuccess, 0)
	e.persistGitInfo(runCtx, run, srcDir, lg)
	e.saveRun(run)

	// ---- 阶段执行 ----
	baseEnv := append(os.Environ(),
		"GIT_URL="+def.Git.URL,
		"GIT_BRANCH="+run.Branch,
		"GIT_COMMIT="+run.Commit,
		"PIPELINE_NAME="+def.Name,
		"RUN_ID="+runID,
		"WORKSPACE="+srcDir,
	)
	for k, v := range def.Env {
		baseEnv = append(baseEnv, k+"="+v)
	}

	workDir := filepath.Join(srcDir, def.Workdir)

	for i := range def.Stages {
		stage := &def.Stages[i]
		st := e.startStage(runID, stage.Name, stage.Type())
		lg.printf("==> 阶段 [%s] 开始", stage.Name)

		stageEnv := append([]string{}, baseEnv...)
		for k, v := range stage.Env {
			stageEnv = append(stageEnv, k+"="+v)
		}

		var (
			exitCode int
			stageErr error
		)
		if stage.IsDocker() {
			exitCode, stageErr = e.runDockerStage(runCtx, stage, workDir, stageEnv, secrets, lg)
		} else {
			exitCode, stageErr = e.runCommandStage(runCtx, stage, workDir, stageEnv, secrets, lg)
		}

		if stageErr != nil {
			if runCtx.Err() != nil || errors.Is(stageErr, context.Canceled) {
				e.endStage(st, model.StatusCanceled, exitCode)
				lg.printf("==> 运行已取消")
				e.finishRun(run, model.StatusCanceled, "手动取消")
				return
			}
			e.endStage(st, model.StatusFailed, exitCode)
			msg := fmt.Sprintf("阶段 [%s] 失败: %v", stage.Name, stageErr)
			e.fail(run, lg, msg)
			lg.printf("工作目录保留在 %s，便于排查", runDir)
			return
		}

		e.endStage(st, model.StatusSuccess, 0)
		lg.printf("==> 阶段 [%s] 完成", stage.Name)
	}

	e.finishRun(run, model.StatusSuccess, "")
	lg.printf("==> 运行 %s 成功结束", runID)

	if e.cfg.Workspace.CleanupOnSuccess {
		if err := os.RemoveAll(runDir); err != nil {
			lg.printf("清理工作目录失败: %v", err)
		} else {
			lg.printf("已清理工作目录 %s", runDir)
		}
	}
}

// checkout 克隆代码仓库并确定分支 / commit。
func (e *Executor) checkout(ctx context.Context, run *model.PipelineRun, def *pipeline.Definition, srcDir string, secrets []string, lg *fileLogger) error {
	cloneURL := def.Git.URL
	if def.Git.Username != "" || def.Git.Password != "" {
		if u, err := url.Parse(def.Git.URL); err == nil {
			u.User = url.UserPassword(def.Git.Username, def.Git.Password)
			cloneURL = u.String()
		}
	}

	args := []string{"clone", "--depth", "1", "--progress"}
	branch := run.Branch
	if branch != "" {
		args = append(args, "--branch", branch)
	}
	args = append(args, cloneURL, srcDir)

	displayArgs := make([]string, len(args))
	copy(displayArgs, args)
	for i := range displayArgs {
		for _, s := range secrets {
			if s != "" {
				displayArgs[i] = strings.ReplaceAll(displayArgs[i], s, "******")
			}
		}
	}
	lg.printf("$ git %s", strings.Join(displayArgs, " "))

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = e.cfg.Workspace.Root
	if err := e.streamCmd(ctx, cmd, lg); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("git clone 失败: %w", err)
	}
	return nil
}

// persistGitInfo 记录检出后的 commit 与提交信息；分支为空时回退为仓库默认分支。
func (e *Executor) persistGitInfo(ctx context.Context, run *model.PipelineRun, srcDir string, lg *fileLogger) {
	commit, err := gitCapture(ctx, srcDir, "rev-parse", "HEAD")
	if err != nil {
		lg.printf("读取 commit 失败: %v", err)
		return
	}
	subject, err := gitCapture(ctx, srcDir, "log", "-1", "--pretty=%s")
	if err != nil {
		subject = ""
	}
	if len(subject) > 500 {
		subject = subject[:500]
	}
	if run.Branch == "" {
		if b, err := gitCapture(ctx, srcDir, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
			run.Branch = b
		}
	}

	run.Commit = commit
	run.Message = subject
	lg.printf("==> commit %s %s", commit, subject)
}

func (e *Executor) runCommandStage(ctx context.Context, stage *pipeline.Stage, dir string, env, secrets []string, lg *fileLogger) (int, error) {
	for _, command := range stage.Command {
		code, err := e.runShell(ctx, dir, env, command, secrets, lg)
		if err != nil {
			return code, err
		}
	}
	return 0, nil
}

func (e *Executor) runDockerStage(ctx context.Context, stage *pipeline.Stage, dir string, env, secrets []string, lg *fileLogger) (int, error) {
	args := []string{"build"}
	if stage.Dockerfile != "" {
		args = append(args, "-f", stage.Dockerfile)
	}
	if stage.Image != "" {
		args = append(args, "-t", stage.Image)
	}
	contextDir := stage.Context
	if contextDir == "" {
		contextDir = "."
	}
	args = append(args, contextDir)

	code, err := e.runShell(ctx, dir, env, "docker "+strings.Join(args, " "), secrets, lg)
	if err != nil {
		return code, err
	}
	if stage.Push && stage.Image != "" {
		code, err = e.runShell(ctx, dir, env, "docker push "+stage.Image, secrets, lg)
		if err != nil {
			return code, err
		}
	}
	return 0, nil
}

// runShell 在系统 shell 中执行命令并实时输出日志。
func (e *Executor) runShell(ctx context.Context, dir string, env []string, command string, secrets []string, lg *fileLogger) (int, error) {
	display := command
	for _, s := range secrets {
		if s != "" {
			display = strings.ReplaceAll(display, s, "******")
		}
	}
	lg.printf("$ %s", display)

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}
	cmd.Dir = dir
	cmd.Env = env

	exitCode, err := e.streamCmdCode(ctx, cmd, lg)
	if err != nil {
		if ctx.Err() != nil {
			lg.printf("（命令已取消）")
			return -1, ctx.Err()
		}
		return exitCode, err
	}
	return 0, nil
}

// streamCmd 执行命令并输出日志，返回错误；适用于不关心退出码的场景。
func (e *Executor) streamCmd(ctx context.Context, cmd *exec.Cmd, lg *fileLogger) error {
	_, err := e.streamCmdCode(ctx, cmd, lg)
	return err
}

// streamCmdCode 执行命令，将 stdout/stderr 流式写入日志，返回退出码。
func (e *Executor) streamCmdCode(ctx context.Context, cmd *exec.Cmd, lg *fileLogger) (int, error) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return -1, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return -1, err
	}

	if err := cmd.Start(); err != nil {
		return -1, fmt.Errorf("启动命令失败: %w", err)
	}

	var wg sync.WaitGroup
	scan := func(r io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for scanner.Scan() {
			lg.line(scanner.Text())
		}
	}
	wg.Add(2)
	go scan(stdout)
	go scan(stderr)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), fmt.Errorf("命令退出码 %d", exitErr.ExitCode())
		}
		return -1, err
	}
	return 0, nil
}

func gitCapture(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (e *Executor) startStage(runID, name, typ string) *model.RunStage {
	now := time.Now()
	st := &model.RunStage{
		RunID:     runID,
		Name:      name,
		Type:      typ,
		Status:    model.StatusRunning,
		StartedAt: &now,
	}
	if err := e.db.Create(st).Error; err != nil {
		log.Printf("create stage %s/%s: %v", runID, name, err)
	}
	return st
}

func (e *Executor) endStage(st *model.RunStage, status string, exitCode int) {
	now := time.Now()
	st.Status = status
	st.ExitCode = exitCode
	st.FinishedAt = &now
	if st.StartedAt != nil {
		st.DurationMs = now.Sub(*st.StartedAt).Milliseconds()
	}
	if err := e.db.Save(st).Error; err != nil {
		log.Printf("save stage %d: %v", st.ID, err)
	}
}

func (e *Executor) saveRun(run *model.PipelineRun) {
	if err := e.db.Save(run).Error; err != nil {
		log.Printf("save run %s: %v", run.ID, err)
	}
}

func (e *Executor) finishRun(run *model.PipelineRun, status, errMsg string) {
	now := time.Now()
	run.Status = status
	run.Error = errMsg
	run.FinishedAt = &now
	e.saveRun(run)
}

func (e *Executor) fail(run *model.PipelineRun, lg *fileLogger, msg string) {
	lg.printf("==> 运行失败: %s", msg)
	e.finishRun(run, model.StatusFailed, msg)
}

type fileLogger struct {
	f *os.File
}

func (l *fileLogger) line(s string) {
	if _, err := fmt.Fprintln(l.f, s); err != nil {
		log.Printf("write log: %v", err)
	}
}

func (l *fileLogger) printf(format string, a ...any) {
	l.line(fmt.Sprintf(format, a...))
}
