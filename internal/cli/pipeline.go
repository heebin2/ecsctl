package cli

import (
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
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
		"  ecs pl list",
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
	pipelineCmd.AddCommand(pipelineListCmd)
	pipelineCmd.AddCommand(pipelineStatusCmd)
	rootCmd.AddCommand(pipelineCmd)
}
