// Package ecssvc는 ECS 클러스터/서비스/태스크 조회 헬퍼를 제공한다.
package ecssvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ListClusters는 계정의 모든 ECS 클러스터 이름(short name)을 반환한다.
func ListClusters(ctx context.Context, c *ecs.Client) ([]string, error) {
	var arns []string
	p := ecs.NewListClustersPaginator(c, &ecs.ListClustersInput{})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("클러스터 목록 조회 실패: %w", err)
		}
		arns = append(arns, out.ClusterArns...)
	}

	names := make([]string, len(arns))
	for i, a := range arns {
		names[i] = shortName(a)
	}
	return names, nil
}

// ListServices는 클러스터의 모든 서비스를 DescribeServices 결과로 반환한다.
func ListServices(ctx context.Context, c *ecs.Client, cluster string) ([]ecstypes.Service, error) {
	var arns []string
	p := ecs.NewListServicesPaginator(c, &ecs.ListServicesInput{Cluster: aws.String(cluster)})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("서비스 목록 조회 실패: %w", err)
		}
		arns = append(arns, out.ServiceArns...)
	}
	return describeServices(ctx, c, cluster, arns)
}

// DescribeService는 단일 서비스를 조회한다.
func DescribeService(ctx context.Context, c *ecs.Client, cluster, service string) (*ecstypes.Service, error) {
	svcs, err := describeServices(ctx, c, cluster, []string{service})
	if err != nil {
		return nil, err
	}
	if len(svcs) == 0 {
		return nil, fmt.Errorf("서비스를 찾을 수 없습니다: %s (cluster: %s)", service, cluster)
	}
	return &svcs[0], nil
}

// describeServices는 ARN/이름 목록을 10개씩 배치로 DescribeServices한다.
func describeServices(ctx context.Context, c *ecs.Client, cluster string, ids []string) ([]ecstypes.Service, error) {
	var result []ecstypes.Service
	for _, batch := range chunk(ids, 10) {
		out, err := c.DescribeServices(ctx, &ecs.DescribeServicesInput{
			Cluster:  aws.String(cluster),
			Services: batch,
		})
		if err != nil {
			return nil, fmt.Errorf("서비스 상세 조회 실패: %w", err)
		}
		result = append(result, out.Services...)
	}
	return result, nil
}

// ListRunningTasks는 서비스의 실행 중 태스크 상세를 반환한다.
func ListRunningTasks(ctx context.Context, c *ecs.Client, cluster, service string) ([]ecstypes.Task, error) {
	var arns []string
	p := ecs.NewListTasksPaginator(c, &ecs.ListTasksInput{
		Cluster:     aws.String(cluster),
		ServiceName: aws.String(service),
	})
	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("태스크 목록 조회 실패: %w", err)
		}
		arns = append(arns, out.TaskArns...)
	}

	var result []ecstypes.Task
	for _, batch := range chunk(arns, 100) {
		out, err := c.DescribeTasks(ctx, &ecs.DescribeTasksInput{
			Cluster: aws.String(cluster),
			Tasks:   batch,
		})
		if err != nil {
			return nil, fmt.Errorf("태스크 상세 조회 실패: %w", err)
		}
		result = append(result, out.Tasks...)
	}
	return result, nil
}

// DescribeTaskDef는 태스크 정의를 조회한다 (로그 설정 해석용).
func DescribeTaskDef(ctx context.Context, c *ecs.Client, taskDef string) (*ecstypes.TaskDefinition, error) {
	out, err := c.DescribeTaskDefinition(ctx, &ecs.DescribeTaskDefinitionInput{
		TaskDefinition: aws.String(taskDef),
	})
	if err != nil {
		return nil, fmt.Errorf("태스크 정의 조회 실패: %w", err)
	}
	return out.TaskDefinition, nil
}

// ShortName은 ARN에서 마지막 세그먼트(리소스 이름)를 반환한다.
func ShortName(arn string) string { return shortName(arn) }

func shortName(arn string) string {
	if i := strings.LastIndex(arn, "/"); i >= 0 {
		return arn[i+1:]
	}
	return arn
}

func chunk[T any](s []T, size int) [][]T {
	var out [][]T
	for i := 0; i < len(s); i += size {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		out = append(out, s[i:end])
	}
	return out
}
