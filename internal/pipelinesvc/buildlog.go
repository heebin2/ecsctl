package pipelinesvc

import (
	"context"
	"fmt"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	cptypes "github.com/aws/aws-sdk-go-v2/service/codepipeline/types"
)

// BuildLog는 파이프라인의 CodeBuild 액션 한 건과 그 CloudWatch 로그 위치를 가리킨다.
type BuildLog struct {
	Stage     string // 스테이지 이름
	Action    string // 액션 이름
	Status    string // 액션 실행 상태 (InProgress/Succeeded/Failed)
	BuildID   string // CodeBuild 빌드 ID (project:uuid)
	LogGroup  string // CloudWatch 로그 그룹
	LogStream string // CloudWatch 로그 스트림
}

// LatestBuildLog는 파이프라인 최신 실행에서 CodeBuild 액션을 찾아 로그 위치를 해석한다.
// 진행 중(InProgress)인 빌드를 우선하고, 없으면 가장 최근에 갱신된 빌드를 고른다.
func LatestBuildLog(ctx context.Context, cp *codepipeline.Client, cb *codebuild.Client, pipeline string) (*BuildLog, error) {
	exec, err := LatestExecution(ctx, cp, pipeline)
	if err != nil {
		return nil, err
	}
	if exec == nil {
		return nil, fmt.Errorf("파이프라인 실행 이력이 없습니다: %s", pipeline)
	}
	execID := aws.ToString(exec.PipelineExecutionId)

	out, err := cp.ListActionExecutions(ctx, &codepipeline.ListActionExecutionsInput{
		PipelineName: aws.String(pipeline),
		Filter: &cptypes.ActionExecutionFilter{
			PipelineExecutionId: aws.String(execID),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("액션 실행 이력 조회 실패(%s): %w", pipeline, err)
	}

	// CodeBuild 액션만 추린다.
	var builds []cptypes.ActionExecutionDetail
	for _, d := range out.ActionExecutionDetails {
		if d.Input != nil && d.Input.ActionTypeId != nil &&
			aws.ToString(d.Input.ActionTypeId.Provider) == "CodeBuild" {
			builds = append(builds, d)
		}
	}
	if len(builds) == 0 {
		return nil, fmt.Errorf("이번 실행(%s)에 CodeBuild 액션이 없습니다", execID)
	}

	// 진행 중인 빌드를 맨 앞으로, 그 외에는 최근 갱신순으로 정렬한다.
	sort.SliceStable(builds, func(i, j int) bool {
		ip := builds[i].Status == cptypes.ActionExecutionStatusInProgress
		jp := builds[j].Status == cptypes.ActionExecutionStatusInProgress
		if ip != jp {
			return ip
		}
		ti, tj := builds[i].LastUpdateTime, builds[j].LastUpdateTime
		if ti == nil || tj == nil {
			return ti != nil
		}
		return ti.After(*tj)
	})

	chosen := builds[0]
	buildID := ""
	if chosen.Output != nil && chosen.Output.ExecutionResult != nil {
		buildID = aws.ToString(chosen.Output.ExecutionResult.ExternalExecutionId)
	}
	if buildID == "" {
		return nil, fmt.Errorf("CodeBuild 빌드 ID를 찾을 수 없습니다 (스테이지=%s, 액션=%s). 빌드가 아직 시작되지 않았을 수 있습니다",
			aws.ToString(chosen.StageName), aws.ToString(chosen.ActionName))
	}

	group, stream, err := buildLogLocation(ctx, cb, buildID)
	if err != nil {
		return nil, err
	}

	return &BuildLog{
		Stage:     aws.ToString(chosen.StageName),
		Action:    aws.ToString(chosen.ActionName),
		Status:    string(chosen.Status),
		BuildID:   buildID,
		LogGroup:  group,
		LogStream: stream,
	}, nil
}

// buildLogLocation은 빌드 ID로 CloudWatch 로그 그룹/스트림을 조회한다.
func buildLogLocation(ctx context.Context, cb *codebuild.Client, buildID string) (group, stream string, err error) {
	out, err := cb.BatchGetBuilds(ctx, &codebuild.BatchGetBuildsInput{
		Ids: []string{buildID},
	})
	if err != nil {
		return "", "", fmt.Errorf("빌드 조회 실패(%s): %w", buildID, err)
	}
	if len(out.Builds) == 0 {
		return "", "", fmt.Errorf("빌드를 찾을 수 없습니다: %s", buildID)
	}
	logs := out.Builds[0].Logs
	if logs == nil || aws.ToString(logs.GroupName) == "" {
		return "", "", fmt.Errorf("빌드(%s)에 CloudWatch 로그가 설정되어 있지 않습니다", buildID)
	}
	return aws.ToString(logs.GroupName), aws.ToString(logs.StreamName), nil
}
