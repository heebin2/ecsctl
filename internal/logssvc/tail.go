// Package logssvc는 ECS 태스크의 awslogs 설정 해석과 CloudWatch Logs tail을 담당한다.
package logssvc

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
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

// Tail은 로그 그룹을 FilterLogEvents로 조회하고, Follow면 주기적으로 폴링한다.
func Tail(ctx context.Context, c *cloudwatchlogs.Client, group string, opts TailOptions) error {
	startMs := time.Now().Add(-opts.Since).UnixMilli()
	seen := make(map[string]struct{})
	limit := opts.Lines // 최초 fetch에만 적용

	for {
		lastTs, err := fetchOnce(ctx, c, group, startMs, seen, limit)
		if err != nil {
			return err
		}
		limit = 0 // 이후 폴링은 전체 출력
		if lastTs >= startMs {
			startMs = lastTs + 1 // 다음 폴링은 마지막 이벤트 이후부터
		}

		if !opts.Follow {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(2 * time.Second):
		}
	}
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
