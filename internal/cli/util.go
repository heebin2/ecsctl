package cli

import "time"

// formatTime은 *time.Time을 로컬 시각 문자열로 변환한다 (nil이면 "-").
func formatTime(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
