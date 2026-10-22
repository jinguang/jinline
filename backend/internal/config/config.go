package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server struct {
		Addr      string `yaml:"addr"`
		StaticDir string `yaml:"static_dir"`
	} `yaml:"server"`

	MySQL struct {
		DSN string `yaml:"dsn"`
	} `yaml:"mysql"`

	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
		QueueKey string `yaml:"queue_key"`
	} `yaml:"redis"`

	Workspace struct {
		Root             string `yaml:"root"`
		PipelinesDir     string `yaml:"pipelines_dir"`
		LogsDir          string `yaml:"logs_dir"`
		CleanupOnSuccess bool   `yaml:"cleanup_on_success"`
	} `yaml:"workspace"`

	Worker struct {
		Concurrency int `yaml:"concurrency"`
	} `yaml:"worker"`
}

// Load 从 YAML 文件加载配置，应用默认值并支持环境变量覆盖。
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	cfg.applyDefaults()
	cfg.overrideFromEnv()
	cfg.normalizePaths()
	return cfg, nil
}

// normalizePaths 将目录配置转换为绝对路径，避免执行器中 cwd 与相对路径叠加。
func (c *Config) normalizePaths() {
	toAbs := func(p string) string {
		if p == "" {
			return p
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return p
		}
		return abs
	}
	c.Server.StaticDir = toAbs(c.Server.StaticDir)
	c.Workspace.Root = toAbs(c.Workspace.Root)
	c.Workspace.PipelinesDir = toAbs(c.Workspace.PipelinesDir)
	c.Workspace.LogsDir = toAbs(c.Workspace.LogsDir)
}

func (c *Config) applyDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.MySQL.DSN == "" {
		c.MySQL.DSN = "root:123456@tcp(127.0.0.1:3306)/jinline?charset=utf8mb4&parseTime=True&loc=Local"
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = "127.0.0.1:6379"
	}
	if c.Redis.QueueKey == "" {
		c.Redis.QueueKey = "jinline:queue:build"
	}
	if c.Workspace.Root == "" {
		c.Workspace.Root = "./data/workspace"
	}
	if c.Workspace.PipelinesDir == "" {
		c.Workspace.PipelinesDir = "./data/pipelines"
	}
	if c.Workspace.LogsDir == "" {
		c.Workspace.LogsDir = "./data/logs"
	}
	if c.Worker.Concurrency <= 0 {
		c.Worker.Concurrency = 2
	}
}

func (c *Config) overrideFromEnv() {
	setStr := func(key string, dst *string) {
		if v := os.Getenv(key); v != "" {
			*dst = v
		}
	}
	setStr("JINLINE_ADDR", &c.Server.Addr)
	setStr("JINLINE_STATIC_DIR", &c.Server.StaticDir)
	setStr("JINLINE_MYSQL_DSN", &c.MySQL.DSN)
	setStr("JINLINE_REDIS_ADDR", &c.Redis.Addr)
	setStr("JINLINE_REDIS_PASSWORD", &c.Redis.Password)
	setStr("JINLINE_QUEUE_KEY", &c.Redis.QueueKey)

	if v := os.Getenv("JINLINE_REDIS_DB"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Redis.DB = n
		}
	}
	if v := os.Getenv("JINLINE_WORKER_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			c.Worker.Concurrency = n
		}
	}
}
