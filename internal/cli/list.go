package cli

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/render"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "ECS 서비스 목록과 상태를 표시",
	Example: "  ecs list\n" +
		"  ecs list -c my-cluster",
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		cluster, err := resolveCluster(ctx)
		if err != nil {
			return err
		}

		svcs, err := ecssvc.ListServices(ctx, clients.ECS, cluster)
		if err != nil {
			return err
		}

		render.Section("Cluster: " + cluster)
		t := render.NewTable("SERVICE", "STATUS", "DESIRED", "RUNNING", "PENDING", "ROLLOUT", "TASKDEF")
		for _, s := range svcs {
			rollout := primaryRollout(s)
			t.Row(
				aws.ToString(s.ServiceName),
				render.Status(aws.ToString(s.Status)),
				s.DesiredCount,
				s.RunningCount,
				s.PendingCount,
				render.Status(rollout),
				ecssvc.ShortName(aws.ToString(s.TaskDefinition)),
			)
		}
		t.Flush()
		return nil
	},
}

// primaryRollout은 PRIMARY 배포의 rolloutState를 반환한다 (없으면 "-").
func primaryRollout(s ecstypes.Service) string {
	for _, d := range s.Deployments {
		if aws.ToString(d.Status) == "PRIMARY" {
			if d.RolloutState != "" {
				return string(d.RolloutState)
			}
			return "-"
		}
	}
	return "-"
}

func init() {
	rootCmd.AddCommand(listCmd)
}
