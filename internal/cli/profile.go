package cli

import (
	"fmt"
	"strings"

	"github.com/heebin2/ecsctl/internal/awsclient"
	"github.com/heebin2/ecsctl/internal/prompt"
	"github.com/heebin2/ecsctl/internal/settings"
	"github.com/spf13/cobra"
)

var profileCmd = &cobra.Command{
	Use:   "profile [name]",
	Short: "AWS 프로필 목록 표시 및 기본 프로필 설정",
	Long: "~/.aws/{config,credentials} 의 프로필 목록을 보여주고(현재 기본값 * 표시),\n" +
		"터미널이면 새 기본 프로필을 골라 ~/.aws/ecs-tools.yml 에 저장한다.\n" +
		"이름을 인자로 주면 해당 프로필로 바로 설정한다. (클러스터는 프로필별로 기억된다)",
	Example: "  ecs profile              # 목록 + (터미널이면) 선택해 저장\n" +
		"  ecs profile my-profile   # 지정 프로필로 설정",
	Args: cobra.MaximumNArgs(1),
	// AWS 연결 없이 로컬 설정만 다루므로 root의 PersistentPreRunE(클라이언트 생성/프로필
	// 해석)를 건너뛰고 저장된 설정만 로드한다.
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		s, err := settings.Load()
		if err != nil {
			return err
		}
		cfg = s
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		profiles, err := awsclient.ListProfiles()
		if err != nil {
			return err
		}
		if len(profiles) == 0 {
			return fmt.Errorf("~/.aws 에 프로필이 없습니다")
		}

		// 이름 인자로 직접 설정
		if len(args) == 1 {
			name := args[0]
			if !contains(profiles, name) {
				return fmt.Errorf("프로필을 찾을 수 없습니다: %s\n사용 가능: %s", name, strings.Join(profiles, ", "))
			}
			if err := saveProfile(name); err != nil {
				return err
			}
			fmt.Printf("기본 프로필을 %s 로 설정했습니다.\n", name)
			return nil
		}

		// 비대화형: 목록만 출력
		if !prompt.IsInteractive() {
			printChoices("profiles", profiles, cfg.Profile)
			return nil
		}

		// 대화형: 현재값 안내 후 선택·저장
		printChoices("profiles", profiles, cfg.Profile)
		choice, err := prompt.Select("기본 프로필 선택", profiles)
		if err != nil {
			return err
		}
		if err := saveProfile(choice); err != nil {
			return err
		}
		fmt.Printf("기본 프로필을 %s 로 설정했습니다.\n", choice)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(profileCmd)
}
