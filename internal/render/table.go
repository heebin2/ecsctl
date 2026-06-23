// Package render는 콘솔 출력(표/색상) 공통 헬퍼를 제공한다.
package render

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
)

// Table은 tabwriter 기반의 정렬된 표 출력을 모은다.
type Table struct {
	w *tabwriter.Writer
}

// NewTable은 헤더를 출력하고 표 작성기를 반환한다.
func NewTable(headers ...string) *Table {
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	t := &Table{w: w}
	cols := make([]any, len(headers))
	for i, h := range headers {
		cols[i] = color.New(color.Bold).Sprint(h)
	}
	t.row(cols...)
	return t
}

// Row는 한 행을 추가한다. 각 셀은 fmt.Sprint로 문자열화된다.
func (t *Table) Row(cells ...any) {
	t.row(cells...)
}

func (t *Table) row(cells ...any) {
	strs := make([]string, len(cells))
	for i, c := range cells {
		strs[i] = fmt.Sprint(c)
	}
	line := ""
	for i, s := range strs {
		if i > 0 {
			line += "\t"
		}
		line += s
	}
	fmt.Fprintln(t.w, line)
}

// Flush는 버퍼링된 표를 실제로 출력한다.
func (t *Table) Flush() {
	_ = t.w.Flush()
}

// Section은 굵은 제목 줄을 출력한다.
func Section(title string) {
	fmt.Println(color.New(color.Bold, color.FgCyan).Sprint(title))
}

// Status는 상태 문자열을 상태값에 따라 색을 입혀 반환한다.
func Status(s string) string {
	switch s {
	case "ACTIVE", "RUNNING", "COMPLETED", "Succeeded", "PRIMARY", "HEALTHY", "InProgress":
		if s == "InProgress" {
			return color.YellowString(s)
		}
		return color.GreenString(s)
	case "FAILED", "STOPPED", "Failed", "UNHEALTHY", "INACTIVE":
		return color.RedString(s)
	case "PENDING", "PROVISIONING", "DEPROVISIONING", "Stopped", "Superseded", "IN_PROGRESS":
		return color.YellowString(s)
	default:
		return s
	}
}
