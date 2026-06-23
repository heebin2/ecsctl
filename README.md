# ecsctl

AWS ECS 서비스 상태 조회, 실시간 로그 추적, CodePipeline 배포 상태 확인을 터미널에서
빠르게 처리하는 CLI. 프로젝트 이름은 **ecsctl**이지만 **`ecs` 명령으로 설치/사용**한다.
모든 명령은 `ecs`로 시작하며 docker/kubectl 스타일을 따른다.

- **인증**: AWS default 자격증명 체인 (`~/.aws`, 환경변수 등)
- **리전**: `ap-northeast-2` (서울) 고정
- **로그 소스**: ECS `awslogs` 드라이버 → CloudWatch Logs

## 빌드 / 설치

> 저장소 이름은 `ecsctl`, 설치되는 실행 파일 이름은 `ecs`다.

```sh
./install.sh          # 빌드 후 /usr/local/bin/ecs 로 설치 (필요 시 sudo)
```

설치 위치를 바꾸려면 `INSTALL_DIR`을 지정한다:

```sh
INSTALL_DIR="$HOME/.local/bin" ./install.sh
```

`go install`로 설치해도 진입점이 `cmd/ecs`라 실행 파일은 `ecs`가 된다:

```sh
go install github.com/heebin2/ecsctl/cmd/ecs@latest   # $GOBIN(또는 ~/go/bin)/ecs
```

직접 빌드만 하려면:

```sh
go build -o ecs ./cmd/ecs
```

### 릴리스 바이너리

[Releases](https://github.com/heebin2/ecsctl/releases) 에서 OS/arch별 tar.gz를 받아 설치한다.

```sh
VER=v1.0.0; OS=darwin; ARCH=arm64   # 예: Apple Silicon
curl -sSL "https://github.com/heebin2/ecsctl/releases/download/${VER}/ecs_${VER}_${OS}_${ARCH}.tar.gz" | tar xz
sudo install -m 0755 ecs /usr/local/bin/ecs && rm ecs
```

## 릴리스 / 배포

`v*` 태그를 push하면 GitHub Actions(`.github/workflows/release.yml`)가 darwin/linux
(amd64·arm64) 바이너리를 빌드해 Release에 업로드한다. 버전은 태그명이 `ecs --version`에 주입된다.

```sh
git tag v1.0.0
git push origin v1.0.0
```

## 사용법

### ECS 서비스 목록

```sh
ecs list                 # 클러스터 자동 선택(1개일 때) 후 서비스 목록
ecs list -c my-cluster   # 클러스터 지정
```

출력: 서비스명 / status / desired·running·pending / 배포 rollout 상태 / task def

### 서비스 상세 상태

```sh
ecs status my-service
ecs status my-service -c my-cluster
```

배포(deployments) 목록, 실행 중 태스크의 lastStatus/health, 최근 서비스 이벤트를 표시한다.

### 실시간 로그 (docker logs 스타일)

```sh
ecs logs my-service                  # 최근 10분 로그
ecs logs my-service -f               # 실시간 추적 (Ctrl+C로 종료)
ecs logs my-service --since 1h       # 최근 1시간
ecs logs my-service --tail 100 -f    # 마지막 100줄 후 실시간 추적
ecs logs my-service -n app           # 특정 컨테이너 선택
```

서비스의 task def에서 `awslogs` 로그 그룹을 자동 해석해 CloudWatch Logs를 조회한다.

| 플래그 | 설명 |
|---|---|
| `-f, --follow` | 실시간 추적 |
| `--since` | 조회 시작 시점 (예: `10m`, `1h`), 기본 `10m` |
| `--tail` | 최초 출력 시 마지막 N줄만 |
| `-n, --container` | 대상 컨테이너 (미지정 시 첫 awslogs 컨테이너) |

### CodePipeline 배포 상태

```sh
ecs pipeline list             # 파이프라인별 최신 실행 상태
ecs pipeline status my-pipe   # 스테이지/액션별 상태 + 실패 사유
```

`pipeline`은 `pl`로 줄여 쓸 수 있다 (`ecs pl list`).

### 프로필 / 클러스터 설정

```sh
ecs profile                 # 프로필 목록(현재 * 표시), 터미널이면 골라서 저장
ecs profile my-profile      # 지정 프로필로 바로 설정
ecs cluster                 # 클러스터 목록(현재 * 표시), 터미널이면 골라서 저장
ecs cluster my-cluster      # 지정 클러스터로 바로 설정
```

선택값은 `~/.aws/ecs-tools.yml` 에 저장된다. 프로필을 바꾸면 저장된 클러스터는 초기화된다.
(`ecs profile`은 AWS 호출 없이 `~/.aws` 만 읽고, `ecs cluster`는 계정의 클러스터를 조회한다.)

## 공통 플래그 / 기본값 저장

| 플래그 | 설명 |
|---|---|
| `-p, --profile` | AWS 프로필 (미지정 시 저장값 또는 목록에서 선택) |
| `-c, --cluster` | 대상 ECS 클러스터 (미지정 시 저장값 또는 목록에서 선택) |

프로필과 클러스터는 **인자로 받지 않으면 목록에서 고르게** 한다.

- 프로필: `~/.aws/{config,credentials}` 의 프로필이 여러 개면 번호로 선택
- 클러스터: 계정에 1개면 자동, 여러 개면 번호로 선택

한 번 고른 값은 `~/.aws/ecs-tools.yml` 에 저장되어 **다음 실행부터 자동 사용**된다.
플래그로 다른 값을 주면 그 값이 새로 저장된다. 프로필을 바꾸면 저장된 클러스터는 초기화된다.

```yaml
# ~/.aws/ecs-tools.yml
profile: default
cluster: my-cluster
```

> 터미널이 아닌 환경(파이프/CI 등)에서는 대화형 선택이 불가능하므로 `-p`/`-c` 로 직접 지정해야 한다.

## 프로젝트 구조

```
cmd/ecs/        진입점 (package main)
internal/cli/   cobra 커맨드 트리 (list/status/logs/pipeline)
internal/
  awsclient/    AWS SDK 설정 및 서비스 클라이언트 생성
  ecssvc/       ECS 클러스터/서비스/태스크 조회 헬퍼
  logssvc/      awslogs 설정 해석 + CloudWatch Logs tail
  pipelinesvc/  CodePipeline 조회 헬퍼
  render/       표/색상 콘솔 출력
```

## 도움말 / 버전

```sh
ecs --help            # 전체 명령/플래그
ecs <command> --help  # 명령별 상세 + 예시
ecs --version         # 버전 (install.sh가 git describe로 주입)
```

## 참고

- `ecs logs <svc>` 는 `--since`/`--tail` 범위의 과거 로그를 `FilterLogEvents`로 한 번
  출력한다. `-f` 를 주면 이어서 CloudWatch `StartLiveTail` 스트리밍으로 새 로그를
  near-real-time 으로 출력한다(Ctrl+C로 종료). `StartLiveTail` 에는 `logs:StartLiveTail`
  권한이 필요하다.
- 자격증명이 만료되면 `ExpiredTokenException` 메시지가 출력된다. SSO 사용 시
  `aws sso login` 후 다시 실행한다.
