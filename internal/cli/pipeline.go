package cli

import (
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/heebin2/ecsctl/internal/logssvc"
	"github.com/heebin2/ecsctl/internal/pipelinesvc"
	"github.com/heebin2/ecsctl/internal/render"
	"github.com/spf13/cobra"
)

var pipelineCmd = &cobra.Command{
	Use:     "pipeline",
	Short:   "CodePipeline 배포 상태 조회",
	Aliases: []string{"pl"},
	Example: "  ecs pipeline list\n" +
		"  ecs pipeline status my-pipe\n" +
		"  ecs pipeline logs my-pipe\n" +
		"  ecs pl list",
}

var (
	pipelineLogsFollow bool
	pipelineLogsSince  time.Duration
	pipelineLogsTail   int
)

var pipelineLogsCmd = &cobra.Command{
	Use:   "logs <name>",
	Short: "파이프라인 최신 실행의 CodeBuild 빌드 로그를 실시간 조회",
	Long: "파이프라인의 가장 최근 실행에서 CodeBuild 액션을 찾아 해당 빌드의\n" +
		"CloudWatch 로그를 출력한다. 진행 중인 빌드가 있으면 그 빌드를 우선하며,\n" +
		"-f 로 실시간 추적한다 (Ctrl+C로 종료).",
	Example: "  ecs pipeline logs my-pipe\n" +
		"  ecs pipeline logs my-pipe -f\n" +
		"  ecs pipeline logs my-pipe --since 30m --tail 200",
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		name := args[0]

		bl, err := pipelinesvc.LatestBuildLog(ctx, clients.CodePipeline, clients.CodeBuild, name)
		if err != nil {
			return err
		}

		render.Section(fmt.Sprintf("pipeline logs: %s (stage=%s, action=%s, status=%s)",
			name, bl.Stage, bl.Action, render.Status(bl.Status)))
		fmt.Printf("build=%s\ngroup=%s stream=%s\n\n", bl.BuildID, bl.LogGroup, bl.LogStream)

		var streams []string
		if bl.LogStream != "" {
			streams = []string{bl.LogStream}
		}

		return logssvc.Tail(ctx, clients.Logs, bl.LogGroup, logssvc.TailOptions{
			Since:   pipelineLogsSince,
			Follow:  pipelineLogsFollow,
			Lines:   pipelineLogsTail,
			Streams: streams,
		})
	},
}

var pipelineListCmd = &cobra.Command{
	Use:   "list",
	Short: "파이프라인 목록과 최신 실행 상태",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		pls, err := pipelinesvc.ListPipelines(ctx, clients.CodePipeline)
		if err != nil {
			return err
		}

		t := render.NewTable("PIPELINE", "LATEST STATUS", "EXECUTION", "UPDATED")
		for _, pl := range pls {
			name := aws.ToString(pl.Name)
			status, exec, updated := "-", "-", "-"
			latest, err := pipelinesvc.LatestExecution(ctx, clients.CodePipeline, name)
			if err != nil {
				return err
			}
			if latest != nil {
				status = render.Status(string(latest.Status))
				exec = aws.ToString(latest.PipelineExecutionId)
				updated = formatTime(latest.LastUpdateTime)
			}
			t.Row(name, status, exec, updated)
		}
		t.Flush()
		return nil
	},
}

var pipelineStatusCmd = &cobra.Command{
	Use:   "status <name>",
	Short: "파이프라인의 스테이지/액션별 상태 (배포 로그)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := cmd.Context()
		name := args[0]

		state, err := pipelinesvc.State(ctx, clients.CodePipeline, name)
		if err != nil {
			return err
		}

		render.Section("Pipeline: " + name)
		for _, stage := range state.StageStates {
			stageStatus := "-"
			if stage.LatestExecution != nil {
				stageStatus = string(stage.LatestExecution.Status)
			}
			fmt.Printf("\n%s  [%s]\n", aws.ToString(stage.StageName), render.Status(stageStatus))

			t := render.NewTable("  ACTION", "STATUS", "LAST CHANGE", "SUMMARY")
			for _, a := range stage.ActionStates {
				status, last, summary := "-", "-", ""
				if le := a.LatestExecution; le != nil {
					status = string(le.Status)
					last = formatTime(le.LastStatusChange)
					summary = aws.ToString(le.Summary)
					if le.ErrorDetails != nil && aws.ToString(le.ErrorDetails.Message) != "" {
						summary = "ERROR: " + aws.ToString(le.ErrorDetails.Message)
					}
				}
				t.Row("  "+aws.ToString(a.ActionName), render.Status(status), last, summary)
			}
			t.Flush()
		}
		return nil
	},
}

func init() {
	pipelineLogsCmd.Flags().BoolVarP(&pipelineLogsFollow, "follow", "f", false, "실시간으로 빌드 로그를 계속 추적")
	pipelineLogsCmd.Flags().DurationVar(&pipelineLogsSince, "since", 1*time.Hour, "조회 시작 시점 (예: 30m, 1h)")
	pipelineLogsCmd.Flags().IntVar(&pipelineLogsTail, "tail", 0, "최초 출력 시 마지막 N줄만 (0이면 since 범위 전체)")

	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineStatusCmd)
	pipelineCmd.AddCommand(pipelineLogsCmd)
	rootCmd.AddCommand(pipelineCmd)
}
