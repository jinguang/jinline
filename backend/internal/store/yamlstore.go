package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"jinline/internal/pipeline"
)

var ErrNotFound = errors.New("流水线不存在")

// YamlStore 将流水线配置以 YAML 文件形式保存在本地目录。
// 文件名即流水线名称：<name>.yaml，写入采用临时文件 + rename 保证原子性。
type YamlStore struct {
	dir string
	mu  sync.RWMutex
}

func NewYamlStore(dir string) (*YamlStore, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("创建流水线配置目录失败: %w", err)
	}
	return &YamlStore{dir: dir}, nil
}

func (s *YamlStore) path(name string) string {
	return filepath.Join(s.dir, name+".yaml")
}

func (s *YamlStore) Exists(name string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, err := os.Stat(s.path(name))
	return err == nil
}

// SaveRaw 保存原始 YAML 内容（保留用户手写格式），def 需已通过解析校验。
func (s *YamlStore) SaveRaw(content []byte, def *pipeline.Definition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	final := s.path(def.Name)
	tmp := final + ".tmp"
	if err := os.WriteFile(tmp, content, 0o644); err != nil {
		return fmt.Errorf("写入流水线配置失败: %w", err)
	}
	if err := os.Rename(tmp, final); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("保存流水线配置失败: %w", err)
	}
	return nil
}

func (s *YamlStore) GetRaw(name string) ([]byte, error) {
	if !pipeline.ValidName(name) {
		return nil, ErrNotFound
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	data, err := os.ReadFile(s.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return data, nil
}

// ModTime 返回配置文件最后修改时间。
func (s *YamlStore) ModTime(name string) time.Time {
	if !pipeline.ValidName(name) {
		return time.Time{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	info, err := os.Stat(s.path(name))
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (s *YamlStore) Get(name string) (*pipeline.Definition, error) {
	data, err := s.GetRaw(name)
	if err != nil {
		return nil, err
	}
	def, err := pipeline.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("流水线配置无效: %w", err)
	}
	return def, nil
}

func (s *YamlStore) List() ([]*pipeline.Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	defs := make([]*pipeline.Definition, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		def, err := pipeline.Parse(data)
		if err != nil {
			// 跳过无法解析的配置文件，避免单个坏文件影响列表
			continue
		}
		defs = append(defs, def)
	}

	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs, nil
}

func (s *YamlStore) Delete(name string) error {
	if !pipeline.ValidName(name) {
		return ErrNotFound
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path(name)); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
