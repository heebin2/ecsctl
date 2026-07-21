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
	// Region은 마지막으로 --region 으로 지정한 AWS 리전을 기억한다.
	// 비어 있으면 AWS 기본 체인(환경변수/프로필)으로 해석한다.
	Region string `yaml:"region,omitempty"`
	// Cluster는 구버전 호환용(프로필별 저장 이전). Load 시 Clusters로 이관된다.
	Cluster string `yaml:"cluster,omitempty"`
	// Clusters는 프로필별로 마지막에 고른 클러스터를 기억한다(키: 프로필명, 빈 프로필은 "").
	Clusters map[string]string `yaml:"clusters,omitempty"`
}

// ClusterFor는 해당 프로필에 대해 기억된 클러스터를 반환한다(없으면 빈 문자열).
func (s *Settings) ClusterFor(profile string) string {
	return s.Clusters[profile]
}

// SetClusterFor는 해당 프로필의 클러스터 선택값을 기억한다.
func (s *Settings) SetClusterFor(profile, cluster string) {
	if s.Clusters == nil {
		s.Clusters = map[string]string{}
	}
	s.Clusters[profile] = cluster
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
	// 구버전(단일 cluster) → 프로필별 맵으로 이관.
	if s.Cluster != "" {
		if s.ClusterFor(s.Profile) == "" {
			s.SetClusterFor(s.Profile, s.Cluster)
		}
		s.Cluster = ""
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
