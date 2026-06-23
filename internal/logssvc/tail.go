// Package logssvc는 ECS 태스크의 awslogs 설정 해석과 CloudWatch Logs tail을 담당한다.
package logssvc

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/fatih/color"
)

// LogConfig는 컨테이너의 awslogs 설정을 담는다.
type LogConfig struct {
	Container    string
	LogGroup     string
	StreamPrefix string
}

// ResolveLogConfig는 태스크 정의에서 awslogs 드라이버 컨테이너의 로그 설정을 찾는다.
// container가 비어 있으면 첫 awslogs 컨테이너를 사용한다.
func ResolveLogConfig(td *ecstypes.TaskDefinition, container string) (*LogConfig, error) {
	var available []string
	for _, cd := range td.ContainerDefinitions {
		name := aws.ToString(cd.Name)
		lc := cd.LogConfiguration
		if lc == nil || lc.LogDriver != ecstypes.LogDriverAwslogs {
			continue
		}
		available = append(available, name)
		if container != "" && name != container {
			continue
		}
		group := lc.Options["awslogs-group"]
		if group == "" {
			return nil, fmt.Errorf("컨테이너 %s 에 awslogs-group 설정이 없습니다", name)
		}
		return &LogConfig{
			Container:    name,
			LogGroup:     group,
			StreamPrefix: lc.Options["awslogs-stream-prefix"],
		}, nil
	}

	if container != "" {
		return nil, fmt.Errorf("컨테이너 %s 의 awslogs 설정을 찾을 수 없습니다 (awslogs 컨테이너: %v)", container, available)
	}
	return nil, fmt.Errorf("awslogs 드라이버를 쓰는 컨테이너가 없습니다")
}

// TailOptions는 tail 동작을 제어한다.
type TailOptions struct {
	Since  time.Duration // 시작 시점 (now - Since)
	Follow bool          // 실시간 추적 여부
	Lines  int           // 최초 출력 시 마지막 N줄만 (0이면 전체)
}

// Tail은 --since/--tail 범위의 과거 로그를 FilterLogEvents로 한 번 출력하고,
// Follow면 이어서 StartLiveTail 스트리밍으로 새 로그를 실시간 출력한다.
func Tail(ctx context.Context, c *cloudwatchlogs.Client, group string, opts TailOptions) error {
	startMs := time.Now().Add(-opts.Since).UnixMilli()
	seen := make(map[string]struct{})

	// 1) 과거 로그 1회 출력 (--tail 지정 시 마지막 N줄만)
	if _, err := fetchOnce(ctx, c, group, startMs, seen, opts.Lines); err != nil {
		return err
	}

	if !opts.Follow {
		return nil
	}

	// 2) 실시간 스트리밍 (near real-time, Ctrl+C로 종료)
	return liveTail(ctx, c, group)
}

// liveTail은 StartLiveTail 세션을 열어 새 로그 이벤트를 실시간으로 출력한다.
func liveTail(ctx context.Context, c *cloudwatchlogs.Client, group string) error {
	arn, err := resolveLogGroupARN(ctx, c, group)
	if err != nil {
		return err
	}

	out, err := c.StartLiveTail(ctx, &cloudwatchlogs.StartLiveTailInput{
		LogGroupIdentifiers: []string{arn},
	})
	if err != nil {
		return fmt.Errorf("실시간 로그 세션 시작 실패: %w", err)
	}
	stream := out.GetStream()
	defer stream.Close()

	dim := color.New(color.FgHiBlack)
	for evt := range stream.Events() {
		u, ok := evt.(*cwtypes.StartLiveTailResponseStreamMemberSessionUpdate)
		if !ok {
			continue // SessionStart 등은 무시
		}
		for _, e := range u.Value.SessionResults {
			ts := time.UnixMilli(aws.ToInt64(e.Timestamp)).Local().Format("15:04:05")
			fmt.Printf("%s %s %s\n", dim.Sprint(ts), dim.Sprintf("[%s]", aws.ToString(e.LogStreamName)), aws.ToString(e.Message))
		}
	}
	if err := stream.Err(); err != nil {
		return fmt.Errorf("실시간 로그 스트림 오류: %w", err)
	}
	return nil
}

// resolveLogGroupARN은 로그 그룹 이름으로 StartLiveTail에 쓸 ARN(끝의 :* 제거)을 찾는다.
func resolveLogGroupARN(ctx context.Context, c *cloudwatchlogs.Client, group string) (string, error) {
	out, err := c.DescribeLogGroups(ctx, &cloudwatchlogs.DescribeLogGroupsInput{
		LogGroupNamePrefix: aws.String(group),
	})
	if err != nil {
		return "", fmt.Errorf("로그 그룹 ARN 조회 실패: %w", err)
	}
	for _, lg := range out.LogGroups {
		if aws.ToString(lg.LogGroupName) != group {
			continue
		}
		arn := aws.ToString(lg.LogGroupArn)
		if arn == "" {
			arn = aws.ToString(lg.Arn)
		}
		return strings.TrimSuffix(arn, ":*"), nil
	}
	return "", fmt.Errorf("로그 그룹을 찾을 수 없습니다: %s", group)
}

// fetchOnce는 startMs 이후 이벤트를 가져와 출력하고, 마지막 타임스탬프를 반환한다.
// limit>0이면 시간순 마지막 limit개만 출력한다.
func fetchOnce(ctx context.Context, c *cloudwatchlogs.Client, group string, startMs int64, seen map[string]struct{}, limit int) (int64, error) {
	input := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(group),
		StartTime:    aws.Int64(startMs),
	}
	p := cloudwatchlogs.NewFilterLogEventsPaginator(c, input)

	type event struct {
		ts     int64
		stream string
		msg    string
		id     string
	}
	var events []event

	for p.HasMorePages() {
		out, err := p.NextPage(ctx)
		if err != nil {
			return startMs, fmt.Errorf("로그 조회 실패: %w", err)
		}
		for _, e := range out.Events {
			id := aws.ToString(e.EventId)
			if _, ok := seen[id]; ok {
				continue
			}
			seen[id] = struct{}{}
			events = append(events, event{
				ts:     aws.ToInt64(e.Timestamp),
				stream: aws.ToString(e.LogStreamName),
				msg:    aws.ToString(e.Message),
				id:     id,
			})
		}
	}

	sort.Slice(events, func(i, j int) bool { return events[i].ts < events[j].ts })

	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}

	lastTs := startMs
	dim := color.New(color.FgHiBlack)
	for _, e := range events {
		ts := time.UnixMilli(e.ts).Local().Format("15:04:05")
		fmt.Printf("%s %s %s\n", dim.Sprint(ts), dim.Sprintf("[%s]", e.stream), e.msg)
		if e.ts > lastTs {
			lastTs = e.ts
		}
	}
	return lastTs, nil
}
