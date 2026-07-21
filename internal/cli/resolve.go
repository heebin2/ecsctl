package cli

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/heebin2/ecsctl/internal/awsclient"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/prompt"
)

// resolveProfile은 사용할 AWS 프로필을 결정한다.
// 우선순위: --profile 플래그 > AWS_PROFILE 환경변수 > 저장값 > 목록에서 대화형 선택.
// 플래그/환경변수/저장값이 ~/.aws 에 실제로 존재하는지 검증한다. 존재하지 않는
// 플래그/환경변수는 에러, 존재하지 않는 저장값은 폐기하고 다시 선택한다.
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

	// AWS_PROFILE 환경변수가 설정돼 있으면 저장값보다 우선. 존재하지 않으면 명확히 에러.
	if env := os.Getenv("AWS_PROFILE"); env != "" {
		if !slices.Contains(profiles, env) {
			return "", fmt.Errorf("AWS_PROFILE %q 가 ~/.aws/config(또는 credentials)에 없습니다. 사용 가능: %s", env, strings.Join(profiles, ", "))
		}
		return env, persistProfile(env)
	}

	// 저장값이 유효하면 그대로 사용. 무효(다른 환경의 잔재 등)하면 폐기하고 다시 선택.
	if cfg.Profile != "" {
		if slices.Contains(profiles, cfg.Profile) {
			return cfg.Profile, nil
		}
		fmt.Printf("저장된 프로필 %q 가 ~/.aws 에 없어 삭제합니다.\n", cfg.Profile)
		delete(cfg.Clusters, cfg.Profile)
		cfg.Profile = ""
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

// persistProfile은 결정된 프로필을 저장한다.
// 클러스터는 프로필별로 따로 기억하므로 프로필이 바뀌어도 비우지 않는다.
func persistProfile(profile string) error {
	if profile == cfg.Profile {
		return nil
	}
	cfg.Profile = profile
	return cfg.Save()
}

// resolveRegion은 사용할 AWS 리전을 결정한다.
// 우선순위: --region 플래그 > 저장값. 둘 다 없으면 "" 를 반환해 AWS 기본 체인
// (AWS_REGION/AWS_DEFAULT_REGION 환경변수, 프로필의 region 설정)에 리전 해석을 맡긴다.
// 플래그가 지정되면 ~/.aws/ecs-tools.yml 에 저장해 다음 실행부터 자동 사용한다.
func resolveRegion() (string, error) {
	if regionFlag != "" {
		return regionFlag, persistRegion(regionFlag)
	}
	return cfg.Region, nil
}

// persistRegion은 결정된 리전을 저장한다(값이 같으면 생략).
func persistRegion(region string) error {
	if region == cfg.Region {
		return nil
	}
	cfg.Region = region
	return cfg.Save()
}

// resolveCluster는 사용할 ECS 클러스터를 결정한다.
// 우선순위: --cluster 플래그 > 저장값 > (1개면 자동, 여러 개면 대화형 선택).
// 결정된 값은 ~/.aws/ecs-tools.yml 에 저장한다.
func resolveCluster(ctx context.Context) (string, error) {
	if clusterFlag != "" {
		return clusterFlag, saveCluster(clusterFlag)
	}
	if c := cfg.ClusterFor(cfg.Profile); c != "" {
		return c, nil
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

// saveCluster는 현재 프로필의 클러스터 선택값을 저장한다(값이 같으면 생략).
func saveCluster(name string) error {
	if cfg.ClusterFor(cfg.Profile) == name {
		return nil
	}
	cfg.SetClusterFor(cfg.Profile, name)
	return cfg.Save()
}

// saveProfile은 프로필 선택값을 저장한다(값이 같으면 생략).
// 클러스터는 프로필별로 따로 기억하므로 건드리지 않는다(다음 사용 시 그 프로필의 저장값이 적용된다).
func saveProfile(name string) error {
	if cfg.Profile == name {
		return nil
	}
	cfg.Profile = name
	return cfg.Save()
}
