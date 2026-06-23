// Package settings는 ~/.aws/ecs-tools.yml 에 저장하는 사용자 기본값(프로필/클러스터)을 다룬다.
package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Settings는 ecs CLI가 기억하는 선택값이다.
type Settings struct {
	Profile string `yaml:"profile"`
	Cluster string `yaml:"cluster"`
}

// Path는 설정 파일 경로(~/.aws/ecs-tools.yml)를 반환한다.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("홈 디렉터리 확인 실패: %w", err)
	}
	return filepath.Join(home, ".aws", "ecs-tools.yml"), nil
}

// Load는 설정을 읽는다. 파일이 없으면 빈 설정을 반환한다.
func Load() (*Settings, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Settings{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("설정 파일 읽기 실패: %w", err)
	}
	var s Settings
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("설정 파일 파싱 실패(%s): %w", path, err)
	}
	return &s, nil
}

// Save는 설정을 파일에 기록한다.
func (s *Settings) Save() error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("설정 디렉터리 생성 실패: %w", err)
	}
	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("설정 직렬화 실패: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("설정 파일 쓰기 실패: %w", err)
	}
	return nil
}
