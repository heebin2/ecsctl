package cli

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

// formatTime은 *time.Time을 로컬 시각 문자열로 변환한다 (nil이면 "-").
func formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}

// contains는 slice에 v가 있는지 반환한다.
func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

// printChoices는 항목 목록을 출력하며 현재 선택값(current)에 * 표시를 붙인다.
func printChoices(title string, items []string, current string) {
	fmt.Println(color.New(color.Bold, color.FgCyan).Sprint(title + ":"))
	for _, it := range items {
		if it == current {
			fmt.Printf("%s %s\n", color.GreenString("*"), it)
		} else {
			fmt.Printf("  %s\n", it)
		}
	}
}
