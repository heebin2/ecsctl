// Command ecs는 AWS ECS / CodePipeline 운영 CLI의 진입점이다.
package main

import "github.com/heebin2/ecsctl/internal/cli"

func main() {
	cli.Execute()
}
