package pipeline

import (
	"errors"
	"fmt"
	"regexp"

	"gopkg.in/yaml.v3"
)

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// Definition 是存储于本地文件系统的流水线 YAML 配置。
type Definition struct {
	APIVersion  string            `yaml:"apiVersion" json:"apiVersion"`
	Name        string            `yaml:"name" json:"name"`
	Description string            `yaml:"description" json:"description"`
	Git         GitSource         `yaml:"git" json:"git"`
	Workdir     string            `yaml:"workdir" json:"workdir"`
	Env         map[string]string `yaml:"env" json:"env"`
	Stages      []Stage           `yaml:"stages" json:"stages"`
}

type GitSource struct {
	URL      string `yaml:"url" json:"url"`
	Branch   string `yaml:"branch" json:"branch"`
	Username string `yaml:"username" json:"username,omitempty"`
	Password string `yaml:"password" json:"-"`
}

// Stage 支持两种类型：
//   - 命令阶段：只配置 commands，在 shell 中依次执行；
//   - 镜像阶段：配置 image / dockerfile / context，执行 docker build（可选 push）。
type Stage struct {
	Name    string            `yaml:"name" json:"name"`
	Command []string          `yaml:"commands" json:"commands,omitempty"`
	Env     map[string]string `yaml:"env" json:"env,omitempty"`
	Image   string            `yaml:"image" json:"image,omitempty"`
	// Dockerfile / Context 相对于 workdir。
	Dockerfile string `yaml:"dockerfile" json:"dockerfile,omitempty"`
	Context    string `yaml:"context" json:"context,omitempty"`
	Push       bool   `yaml:"push" json:"push,omitempty"`
}

// IsDocker 判断该阶段是否执行容器镜像构建。
func (s Stage) IsDocker() bool {
	return s.Image != "" || s.Dockerfile != ""
}

// Type 返回阶段的展示类型。
func (s Stage) Type() string {
	if s.IsDocker() {
		return "docker"
	}
	return "command"
}

// Parse 解析并校验 YAML 配置。
func Parse(data []byte) (*Definition, error) {
	def := &Definition{}
	if err := yaml.Unmarshal(data, def); err != nil {
		return nil, fmt.Errorf("YAML 语法错误: %w", err)
	}
	if err := def.Validate(); err != nil {
		return nil, err
	}
	return def, nil
}

func (d *Definition) Validate() error {
	if d.Name == "" {
		return errors.New("name 不能为空")
	}
	if !nameRe.MatchString(d.Name) {
		return errors.New("name 只能包含字母、数字、中划线或下划线，且以字母或数字开头，最长 64 个字符")
	}
	if d.Git.URL == "" {
		return errors.New("git.url 不能为空")
	}
	if len(d.Stages) == 0 {
		return errors.New("至少需要配置一个 stage")
	}

	seen := make(map[string]struct{}, len(d.Stages))
	for i, s := range d.Stages {
		if s.Name == "" {
			return fmt.Errorf("stages[%d].name 不能为空", i)
		}
		if _, ok := seen[s.Name]; ok {
			return fmt.Errorf("stage 名称重复: %s", s.Name)
		}
		seen[s.Name] = struct{}{}

		if s.IsDocker() && s.Image == "" {
			return fmt.Errorf("stage [%s]: 镜像构建阶段必须配置 image", s.Name)
		}
		if !s.IsDocker() && len(s.Command) == 0 {
			return fmt.Errorf("stage [%s]: 需要配置 commands，或配置 image 执行镜像构建", s.Name)
		}
	}

	if d.Workdir == "" {
		d.Workdir = "."
	}
	for i := range d.Stages {
		s := &d.Stages[i]
		if !s.IsDocker() {
			continue
		}
		if s.Dockerfile == "" {
			s.Dockerfile = "Dockerfile"
		}
		if s.Context == "" {
			s.Context = "."
		}
	}
	return nil
}

// ValidName 用于校验来自 URL 的流水线名称，防止路径穿越。
func ValidName(name string) bool {
	return nameRe.MatchString(name)
}
