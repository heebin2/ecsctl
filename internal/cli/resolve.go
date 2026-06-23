package cli

import (
	"context"
	"fmt"

	"github.com/heebin2/ecsctl/internal/awsclient"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/prompt"
)

// resolveProfile은 사용할 AWS 프로필을 결정한다.
// 우선순위: --profile 플래그 > 저장값 > 목록에서 대화형 선택.
// 새로 결정된 값은 ~/.aws/ecs-tools.yml 에 저장한다(프로필이 바뀌면 저장된 클러스터는 무효화).
func resolveProfile() (string, error) {
	profile := profileFlag
	if profile == "" {
		profile = cfg.Profile
	}

	if profile == "" {
		profiles, err := awsclient.ListProfiles()
		if err != nil {
			return "", err
		}
		switch len(profiles) {
		case 0:
			// 프로필 정의가 없으면 default 자격증명 체인에 맡긴다.
			return "", nil
		case 1:
			profile = profiles[0]
		default:
			profile, err = prompt.Select("AWS 프로필 선택", profiles)
			if err != nil {
				return "", err
			}
		}
	}

	if profile != cfg.Profile {
		cfg.Profile = profile
		cfg.Cluster = "" // 프로필이 바뀌면 클러스터는 다시 선택
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}
	return profile, nil
}

// resolveCluster는 사용할 ECS 클러스터를 결정한다.
// 우선순위: --cluster 플래그 > 저장값 > (1개면 자동, 여러 개면 대화형 선택).
// 결정된 값은 ~/.aws/ecs-tools.yml 에 저장한다.
func resolveCluster(ctx context.Context) (string, error) {
	if clusterFlag != "" {
		return clusterFlag, saveCluster(clusterFlag)
	}
	if cfg.Cluster != "" {
		return cfg.Cluster, nil
	}

	clusters, err := ecssvc.ListClusters(ctx, clients.ECS)
	if err != nil {
		return "", err
	}
	switch len(clusters) {
	case 0:
		return "", fmt.Errorf("클러스터가 없습니다")
	case 1:
		return clusters[0], saveCluster(clusters[0])
	default:
		choice, err := prompt.Select("ECS 클러스터 선택", clusters)
		if err != nil {
			return "", err
		}
		return choice, saveCluster(choice)
	}
}

// saveCluster는 클러스터 선택값을 저장한다(값이 같으면 생략).
func saveCluster(name string) error {
	if cfg.Cluster == name {
		return nil
	}
	cfg.Cluster = name
	return cfg.Save()
}
