// Package cli는 ecs CLI의 cobra 커맨드 트리를 구성한다.
package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/heebin2/ecsctl/internal/awsclient"
	"github.com/heebin2/ecsctl/internal/settings"
	"github.com/spf13/cobra"
)

// clients는 PersistentPreRunE에서 초기화되어 모든 서브커맨드가 공유한다.
var clients *awsclient.Clients

// cfg는 ~/.aws/ecs-tools.yml 에서 로드한 저장된 기본값이다.
var cfg *settings.Settings

// clusterFlag/profileFlag는 여러 명령에서 공통으로 쓰는 --cluster/--profile 값이다.
var (
	clusterFlag string
	profileFlag string
)

// version은 빌드 시 -ldflags 로 주입된다(install.sh의 git describe). 미주입 시 "dev".
var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "ecs",
	Short:   "AWS ECS / CodePipeline 운영 CLI (프로젝트명: ecsctl)",
	Version: version,
	Long: "ecs는 AWS ECS 서비스 상태 조회, 실시간 로그 추적, CodePipeline 배포 상태\n" +
		"확인을 터미널에서 빠르게 처리하는 CLI다. (region: " + awsclient.DefaultRegion + ")",
	Example: "  ecs list\n" +
		"  ecs status my-service\n" +
		"  ecs logs my-service -f\n" +
		"  ecs pipeline status my-pipe",
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		s, err := settings.Load()
		if err != nil {
			return err
		}
		cfg = s

		profile, err := resolveProfile()
		if err != nil {
			return err
		}

		c, err := awsclient.New(cmd.Context(), profile)
		if err != nil {
			return err
		}
		clients = c
		return nil
	},
}

// Execute는 루트 커맨드를 실행한다. main에서 호출한다.
func Execute() {
	if err := rootCmd.ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&clusterFlag, "cluster", "c", "", "대상 ECS 클러스터 (미지정 시 저장값 또는 목록에서 선택)")
	rootCmd.PersistentFlags().StringVarP(&profileFlag, "profile", "p", "", "AWS 프로필 (미지정 시 저장값 또는 목록에서 선택)")
}
