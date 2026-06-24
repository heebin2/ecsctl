package cli

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
	"github.com/heebin2/ecsctl/internal/ecssvc"
	"github.com/heebin2/ecsctl/internal/render"
	"github.com/spf13/cobra"
)

var (
	watchInterval time.Duration
	watchTimeout  time.Duration
)

var watchCmd = &cobra.Command{
	Use:   "watch <service>",
	Short: "ECS 서비스의 롤링 배포를 steady state/실패까지 실시간 추적",
	Long: "DescribeServices를 주기적으로 폴링하며 PRIMARY 배포의 롤아웃 상태, 태스크 수,\n" +
		"새 서비스 이벤트를 실시간 출력한다. 배포가 steady state(COMPLETED)에 도달하면\n" +
		"0으로, 서킷 브레이커로 FAILED 되면 0이 아닌 코드로 종료하므로 CI 파이프라인의\n" +
		"배포 검증 단계로 그대로 쓸 수 있다. 실패 시 중지된 태스크의 원인과 컨테이너\n" +
		"종료 코드를 자동으로 보여준다 (Ctrl+C로 종료).",
	Aliases: []string{"w"},
	Example: "  ecs watch my-service\n" +
		"  ecs watch my-service -c my-cluster --interval 3s\n" +
		"  ecs watch my-service --timeout 10m",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		service := args[0]

		cluster, err := resolveCluster(ctx)
		if err != nil {
			return err
		}
		return watchDeployment(ctx, cluster, service)
	},
}

// watchDeployment는 서비스의 PRIMARY 배포가 종료 상태에 도달할 때까지 폴링한다.
func watchDeployment(ctx context.Context, cluster, service string) error {
	render.Section(fmt.Sprintf("Watching deployment: %s  (cluster: %s)", service, cluster))

	deadline := time.Now().Add(watchTimeout)
	seen := make(map[string]struct{})
	first := true
	var lastLine string

	tick := time.NewTicker(watchInterval)
	defer tick.Stop()

	for {
		svc, err := ecssvc.DescribeService(ctx, clients.ECS, cluster, service)
		if err != nil {
			return err
		}

		printNewEvents(svc.Events, seen, first)
		first = false

		primary := primaryDeployment(svc)
		lastLine = printStatusLine(svc, primary, lastLine)

		done, failed := deploymentSettled(svc, primary)
		switch {
		case failed:
			return deploymentFailed(ctx, cluster, service, primary)
		case done:
			render.Section("✓ 배포 완료 (steady state 도달)")
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("배포 감시 시간 초과(%s): 아직 steady state에 도달하지 않았습니다", watchTimeout)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

// deploymentSettled는 PRIMARY 배포가 종료 상태인지 판정한다.
// 서킷 브레이커가 활성이면 RolloutState로, 아니면 고전적 steady-state 휴리스틱
// (단일 배포 + running==desired + pending==0)으로 판단한다.
func deploymentSettled(svc *ecstypes.Service, primary *ecstypes.Deployment) (done, failed bool) {
	if primary == nil {
		return false, false
	}
	switch primary.RolloutState {
	case ecstypes.DeploymentRolloutStateCompleted:
		return true, false
	case ecstypes.DeploymentRolloutStateFailed:
		return false, true
	case ecstypes.DeploymentRolloutStateInProgress:
		return false, false
	}
	// RolloutState 미설정(서킷 브레이커 비활성) → 휴리스틱
	if len(svc.Deployments) == 1 &&
		primary.RunningCount == primary.DesiredCount &&
		primary.PendingCount == 0 {
		return true, false
	}
	return false, false
}

// primaryDeployment는 서비스의 PRIMARY 배포를 반환한다 (없으면 nil).
func primaryDeployment(svc *ecstypes.Service) *ecstypes.Deployment {
	for i := range svc.Deployments {
		if aws.ToString(svc.Deployments[i].Status) == "PRIMARY" {
			return &svc.Deployments[i]
		}
	}
	return nil
}

// printStatusLine은 상태 한 줄을 출력하되 직전과 동일하면 생략한다 (노이즈 억제).
func printStatusLine(svc *ecstypes.Service, primary *ecstypes.Deployment, last string) string {
	rollout := "-"
	reason := ""
	var failedCount int32
	if primary != nil {
		if primary.RolloutState != "" {
			rollout = string(primary.RolloutState)
		}
		reason = aws.ToString(primary.RolloutStateReason)
		failedCount = primary.FailedTasks
	}

	line := fmt.Sprintf("desired=%d running=%d pending=%d failed=%d deployments=%d",
		svc.DesiredCount, svc.RunningCount, svc.PendingCount, failedCount, len(svc.Deployments))
	key := rollout + " " + line + " " + reason
	if key == last {
		return last
	}

	ts := color.New(color.FgHiBlack).Sprint(time.Now().Local().Format("15:04:05"))
	fmt.Printf("%s  %s  %s\n", ts, render.Status(rollout), line)
	if reason != "" {
		fmt.Printf("          reason: %s\n", reason)
	}
	return key
}

// printNewEvents는 아직 보지 않은 서비스 이벤트만 시간순으로 출력한다.
// 최초 폴링에서는 직전 맥락 일부만 보여주고 나머지는 음소거한다(seen 처리).
func printNewEvents(events []ecstypes.ServiceEvent, seen map[string]struct{}, first bool) {
	var fresh []ecstypes.ServiceEvent
	for _, e := range events {
		id := aws.ToString(e.Id)
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		fresh = append(fresh, e)
	}
	sort.Slice(fresh, func(i, j int) bool {
		return aws.ToTime(fresh[i].CreatedAt).Before(aws.ToTime(fresh[j].CreatedAt))
	})
	if first && len(fresh) > 5 {
		fresh = fresh[len(fresh)-5:]
	}

	dim := color.New(color.FgHiBlack)
	for _, e := range fresh {
		ts := aws.ToTime(e.CreatedAt).Local().Format("15:04:05")
		fmt.Printf("%s %s %s\n", dim.Sprint(ts), dim.Sprint("[event]"), aws.ToString(e.Message))
	}
}

// deploymentFailed는 실패 사유와 최근 중지된 태스크의 원인/종료 코드를 출력하고 에러를 반환한다.
func deploymentFailed(ctx context.Context, cluster, service string, primary *ecstypes.Deployment) error {
	render.Section("✗ 배포 실패 (롤아웃 FAILED)")
	if r := aws.ToString(primary.RolloutStateReason); r != "" {
		fmt.Printf("  reason: %s\n", r)
	}

	stopped, err := ecssvc.ListStoppedTasks(ctx, clients.ECS, cluster, service)
	if err != nil {
		fmt.Printf("  (중지된 태스크 조회 실패: %v)\n", err)
		return fmt.Errorf("배포가 실패했습니다: %s", service)
	}

	// 가장 최근에 중지된 태스크 우선, 최대 5개만 출력.
	sort.Slice(stopped, func(i, j int) bool {
		return aws.ToTime(stopped[i].StoppedAt).After(aws.ToTime(stopped[j].StoppedAt))
	})
	if len(stopped) > 5 {
		stopped = stopped[:5]
	}

	if len(stopped) > 0 {
		fmt.Println()
		render.Section("최근 중지된 태스크")
		for _, t := range stopped {
			fmt.Printf("  %s  stopped: %s\n",
				ecssvc.ShortName(aws.ToString(t.TaskArn)), aws.ToString(t.StoppedReason))
			for _, c := range t.Containers {
				exit := "-"
				if c.ExitCode != nil {
					exit = fmt.Sprintf("%d", aws.ToInt32(c.ExitCode))
				}
				fmt.Printf("      - %s exit=%s %s\n",
					aws.ToString(c.Name), exit, aws.ToString(c.Reason))
			}
		}
	}
	return fmt.Errorf("배포가 실패했습니다: %s", service)
}

func init() {
	watchCmd.Flags().DurationVar(&watchInterval, "interval", 5*time.Second, "폴링 간격 (예: 3s, 10s)")
	watchCmd.Flags().DurationVar(&watchTimeout, "timeout", 15*time.Minute, "steady state 대기 최대 시간 (초과 시 실패 처리)")
	rootCmd.AddCommand(watchCmd)
}
