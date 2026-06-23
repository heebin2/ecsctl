// Package pipelinesvc는 CodePipeline 조회 헬퍼를 제공한다.
package pipelinesvc

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// ListPipelines는 계정의 모든 파이프라인 요약을 반환한다.
func ListPipelines(ctx context.Context, c *codepipeline.Client) ([]cptypes.PipelineSummary, error) {
	var result []cptypes.PipelineSummary
	p := codepipeline.NewListPipelinesPaginator(c, &codepipeline.ListPipelinesInput{})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("파이프라인 목록 조회 실패: %w", err)
		}
		result = append(result, out.Pipelines...)
	}
	return result, nil
}

// LatestExecution은 파이프라인의 가장 최근 실행 요약을 반환한다 (없으면 nil).
func LatestExecution(ctx context.Context, c *codepipeline.Client, name string) (*cptypes.PipelineExecutionSummary, error) {
	out, err := c.ListPipelineExecutions(ctx, &codepipeline.ListPipelineExecutionsInput{
		PipelineName: aws.String(name),
		MaxResults:   aws.Int32(1),
	})
	if err != nil {
		return nil, fmt.Errorf("파이프라인 실행 조회 실패(%s): %w", name, err)
	}
	if len(out.PipelineExecutionSummaries) == 0 {
		return nil, nil
	}
	return &out.PipelineExecutionSummaries[0], nil
}

// State는 파이프라인의 스테이지/액션별 상태를 반환한다.
func State(ctx context.Context, c *codepipeline.Client, name string) (*codepipeline.GetPipelineStateOutput, error) {
	out, err := c.GetPipelineState(ctx, &codepipeline.GetPipelineStateInput{
		Name: aws.String(name),
	})
	if err != nil {
		return nil, fmt.Errorf("파이프라인 상태 조회 실패(%s): %w", name, err)
	}
	return out, nil
}
