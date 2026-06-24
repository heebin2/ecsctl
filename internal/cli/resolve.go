package cli

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/heebin2/ecsctl/internal/awsclient"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/prompt"
)

// resolveProfile은 사용할 AWS 프로필을 결정한다.
// 우선순위: --profile 플래그 > 저장값 > 목록에서 대화형 선택.
// 플래그/저장값이 ~/.aws 에 실제로 존재하는지 검증한다. 존재하지 않는
// 플래그는 에러, 존재하지 않는 저장값은 폐기하고 다시 선택한다.
// 새로 결정된 값은 ~/.aws/ecs-tools.yml 에 저장한다(프로필이 바뀌면 저장된 클러스터는 무효화).
func resolveProfile() (string, error) {
	profiles, err := awsclient.ListProfiles()
	if err != nil {
		return "", err
	}

	// 플래그가 지정됐으면 최우선. 존재하지 않으면 명확히 에러.
	if profileFlag != "" {
		if !slices.Contains(profiles, profileFlag) {
			return "", fmt.Errorf("프로필 %q 가 ~/.aws/config(또는 credentials)에 없습니다. 사용 가능: %s", profileFlag, strings.Join(profiles, ", "))
		}
		return profileFlag, persistProfile(profileFlag)
	}

	// 저장값이 유효하면 그대로 사용. 무효(다른 환경의 잔재 등)하면 폐기하고 다시 선택.
	if cfg.Profile != "" {
		if slices.Contains(profiles, cfg.Profile) {
			return cfg.Profile, nil
		}
		fmt.Printf("저장된 프로필 %q 가 ~/.aws 에 없어 삭제합니다.\n", cfg.Profile)
		cfg.Profile = ""
		cfg.Cluster = ""
		if err := cfg.Save(); err != nil {
			return "", err
		}
	}

	var profile string
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
	return profile, persistProfile(profile)
}

// persistProfile은 결정된 프로필을 저장한다(값이 바뀌면 클러스터는 무효화).
func persistProfile(profile string) error {
	if profile == cfg.Profile {
		return nil
	}
	cfg.Profile = profile
	cfg.Cluster = "" // 프로필이 바뀌면 클러스터는 다시 선택
	return cfg.Save()
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

// saveProfile은 프로필 선택값을 저장한다(값이 같으면 생략).
// 프로필이 바뀌면 저장된 클러스터는 초기화한다(resolveProfile과 동일 규칙).
func saveProfile(name string) error {
	if cfg.Profile == name {
		return nil
	}
	cfg.Profile = name
	cfg.Cluster = ""
	return cfg.Save()
}
