package cli

import (
	"fmt"
	"strings"

	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/prompt"
	"github.com/spf13/cobra"
)

var clusterCmd = &cobra.Command{
	Use:   "cluster [name]",
	Short: "ECS 클러스터 목록 표시 및 기본 클러스터 설정",
	Long: "인자 없이 실행하면 클러스터 목록을 보여주고(현재 기본값 * 표시),\n" +
		"터미널이면 새 기본 클러스터를 골라 ~/.aws/ecs-tools.yml 에 저장한다.\n" +
		"이름을 인자로 주면 해당 클러스터로 바로 설정한다.",
	Example: "  ecs cluster              # 목록 + (터미널이면) 선택해 저장\n" +
		"  ecs cluster my-cluster   # 지정 클러스터로 설정",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		clusters, err := ecssvc.ListClusters(ctx, clients.ECS)
		if err != nil {
			return err
		}
		if len(clusters) == 0 {
			return fmt.Errorf("클러스터가 없습니다")
		}

		// 이름 인자로 직접 설정
		if len(args) == 1 {
			name := args[0]
			if !contains(clusters, name) {
				return fmt.Errorf("클러스터를 찾을 수 없습니다: %s\n사용 가능: %s", name, strings.Join(clusters, ", "))
			}
			if err := saveCluster(name); err != nil {
				return err
			}
			fmt.Printf("기본 클러스터를 %s 로 설정했습니다.\n", name)
			return nil
		}

		// 비대화형: 목록만 출력
		if !prompt.IsInteractive() {
			printChoices("clusters", clusters, cfg.Cluster)
			return nil
		}

		// 대화형: 현재값 안내 후 선택·저장
		printChoices("clusters", clusters, cfg.Cluster)
		choice, err := prompt.Select("기본 클러스터 선택", clusters)
		if err != nil {
			return err
		}
		if err := saveCluster(choice); err != nil {
			return err
		}
		fmt.Printf("기본 클러스터를 %s 로 설정했습니다.\n", choice)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(clusterCmd)
}
