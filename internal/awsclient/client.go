// Package awsclient는 AWS SDK 설정과 서비스 클라이언트 생성을 담당한다.
package awsclient

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

// DefaultRegion은 사용자 환경(서울 리전)에 맞춘 기본 리전이다.
const DefaultRegion = "ap-northeast-2"

// Clients는 CLI 전반에서 공유하는 AWS 서비스 클라이언트 묶음이다.
type Clients struct {
	Config       aws.Config
	ECS          *ecs.Client
	Logs         *cloudwatchlogs.Client
	CodePipeline *codepipeline.Client
}

// New는 DefaultRegion과 지정한 프로필로 모든 서비스 클라이언트를 생성한다.
// profile이 비어 있으면 default 자격증명 체인을 사용한다.
func New(ctx context.Context, profile string) (*Clients, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(DefaultRegion)}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("AWS 설정 로드 실패: %w", err)
	}

	return &Clients{
		Config:       cfg,
		ECS:          ecs.NewFromConfig(cfg),
		Logs:         cloudwatchlogs.NewFromConfig(cfg),
		CodePipeline: codepipeline.NewFromConfig(cfg),
	}, nil
}

// ListProfiles는 ~/.aws/config 와 ~/.aws/credentials 에 정의된 프로필 이름을 반환한다.
func ListProfiles() ([]string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("홈 디렉터리 확인 실패: %w", err)
	}

	set := map[string]struct{}{}
	// config: [default], [profile NAME] (sso-session/services 섹션은 제외)
	if err := scanProfiles(filepath.Join(home, ".aws", "config"), set, true); err != nil {
		return nil, err
	}
	// credentials: [NAME]
	if err := scanProfiles(filepath.Join(home, ".aws", "credentials"), set, false); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(set))
	for n := range set {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		// default를 맨 앞으로
		if names[i] == "default" {
			return true
		}
		if names[j] == "default" {
			return false
		}
		return names[i] < names[j]
	})
	return names, nil
}

// scanProfiles는 ini 형식 파일에서 프로필 섹션 이름을 set에 추가한다.
// isConfig면 config 파일 규칙([profile X])을, 아니면 credentials 규칙([X])을 적용한다.
func scanProfiles(path string, set map[string]struct{}, isConfig bool) error {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s 읽기 실패: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "]") {
			continue
		}
		section := strings.TrimSpace(line[1 : len(line)-1])
		if isConfig {
			switch {
			case section == "default":
				set[section] = struct{}{}
			case strings.HasPrefix(section, "profile "):
				set[strings.TrimSpace(strings.TrimPrefix(section, "profile "))] = struct{}{}
			}
			continue
		}
		set[section] = struct{}{}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("%s 스캔 실패: %w", path, err)
	}
	return nil
}
