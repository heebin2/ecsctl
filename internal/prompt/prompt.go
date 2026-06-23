// Package prompt는 터미널에서의 간단한 번호 선택 UI를 제공한다.
package prompt

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/mattn/go-isatty"
)

// Select는 options를 번호와 함께 출력하고 사용자가 고른 항목을 반환한다.
// stdin이 터미널이 아니면(파이프/CI 등) 에러를 반환한다.
func Select(label string, options []string) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("선택할 항목이 없습니다")
	}
	if !isatty.IsTerminal(os.Stdin.Fd()) {
		return "", fmt.Errorf("대화형 선택이 불가능합니다(터미널 아님). 플래그로 직접 지정하세요")
	}

	fmt.Fprintln(os.Stderr, label+":")
	for i, opt := range options {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, opt)
	}

	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Fprintf(os.Stderr, "번호 선택 [1-%d]: ", len(options))
		line, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("입력 읽기 실패: %w", err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > len(options) {
			fmt.Fprintln(os.Stderr, "  올바른 번호를 입력하세요.")
			continue
		}
		return options[n-1], nil
	}
}
