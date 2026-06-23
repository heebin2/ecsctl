package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/render"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status <service>",
	Short: "단일 ECS 서비스의 배포/이벤트/태스크 상세",
	Example: "  ecs status my-service\n" +
		"  ecs status my-service -c my-cluster",
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

		render.Section(fmt.Sprintf("Service: %s  (cluster: %s)", aws.ToString(svc.ServiceName), cluster))
		fmt.Printf("  status=%s  desired=%d  running=%d  pending=%d  taskDef=%s\n\n",
			render.Status(aws.ToString(svc.Status)),
			svc.DesiredCount, svc.RunningCount, svc.PendingCount,
			ecssvc.ShortName(aws.ToString(svc.TaskDefinition)))

		// 배포
		render.Section("Deployments")
		dt := render.NewTable("STATUS", "ROLLOUT", "DESIRED", "RUNNING", "PENDING", "FAILED", "TASKDEF", "UPDATED")
		for _, d := range svc.Deployments {
			rollout := string(d.RolloutState)
			if rollout == "" {
				rollout = "-"
			}
			dt.Row(
				render.Status(aws.ToString(d.Status)),
				render.Status(rollout),
				d.DesiredCount, d.RunningCount, d.PendingCount, d.FailedTasks,
				ecssvc.ShortName(aws.ToString(d.TaskDefinition)),
				formatTime(d.UpdatedAt),
			)
		}
		dt.Flush()

		// rolloutStateReason (있으면)
		for _, d := range svc.Deployments {
			if aws.ToString(d.Status) == "PRIMARY" && aws.ToString(d.RolloutStateReason) != "" {
				fmt.Printf("  reason: %s\n", aws.ToString(d.RolloutStateReason))
			}
		}

		// 실행 중 태스크
		tasks, err := ecssvc.ListRunningTasks(ctx, clients.ECS, cluster, service)
		if err != nil {
			return err
		}
		fmt.Println()
		render.Section(fmt.Sprintf("Tasks (%d)", len(tasks)))
		tt := render.NewTable("TASK", "LAST", "DESIRED", "HEALTH", "STARTED")
		for _, t := range tasks {
			tt.Row(
				ecssvc.ShortName(aws.ToString(t.TaskArn)),
				render.Status(aws.ToString(t.LastStatus)),
				aws.ToString(t.DesiredStatus),
				render.Status(string(t.HealthStatus)),
				formatTime(t.StartedAt),
			)
		}
		tt.Flush()

		// 최근 이벤트 (최신 10개)
		fmt.Println()
		render.Section("Recent events")
		n := len(svc.Events)
		max := 10
		if n < max {
			max = n
		}
		for i := 0; i < max; i++ {
			e := svc.Events[i] // Events는 최신순으로 반환됨
			fmt.Printf("  %s  %s\n", formatTime(e.CreatedAt), aws.ToString(e.Message))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
