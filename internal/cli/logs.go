package cli

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/logssvc"
	"github.com/heebin2/ecsctl/internal/render"
	"github.com/spf13/cobra"
)

var (
	logsFollow    bool
	logsSince     time.Duration
	logsTail      int
	logsContainer string
)

var logsCmd = &cobra.Command{
	Use:   "logs <service>",
	Short: "ECS 서비스의 CloudWatch 로그를 조회/추적 (docker logs 스타일)",
	Example: "  ecs logs my-service\n" +
		"  ecs logs my-service -f\n" +
		"  ecs logs my-service --since 1h --tail 100\n" +
		"  ecs logs my-service -n app -f",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		service := args[0]

		cluster, err := resolveCluster(ctx)
		if err != nil {
			return err
		}

		svc, err := ecssvc.DescribeService(ctx, clients.ECS, cluster, service)
		if err != nil {
			return err
		}

		td, err := ecssvc.DescribeTaskDef(ctx, clients.ECS, aws.ToString(svc.TaskDefinition))
		if err != nil {
			return err
		}

		lc, err := logssvc.ResolveLogConfig(td, logsContainer)
		if err != nil {
			return err
		}

		render.Section(fmt.Sprintf("logs: %s (container=%s, group=%s)", service, lc.Container, lc.LogGroup))

		return logssvc.Tail(ctx, clients.Logs, lc.LogGroup, logssvc.TailOptions{
			Since:  logsSince,
			Follow: logsFollow,
			Lines:  logsTail,
		})
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsFollow, "follow", "f", false, "실시간으로 로그를 계속 추적")
	logsCmd.Flags().DurationVar(&logsSince, "since", 10*time.Minute, "조회 시작 시점 (예: 10m, 1h)")
	logsCmd.Flags().IntVar(&logsTail, "tail", 0, "최초 출력 시 마지막 N줄만 (0이면 since 범위 전체)")
	logsCmd.Flags().StringVarP(&logsContainer, "container", "n", "", "대상 컨테이너 (미지정 시 첫 awslogs 컨테이너)")
	rootCmd.AddCommand(logsCmd)
}
