# jinline

一个轻量的 CI/CD 流水线系统：从 Git 仓库拉取代码、在本地编译构建、可选的容器镜像构建，并提供 Web 界面查看流水线配置、运行状态与实时日志。

## 技术栈

| 模块 | 选型 |
| --- | --- |
| 后端 | Go + Gin |
| 状态存储 | MySQL（GORM，流水线运行/阶段状态） |
| 消息队列 | Redis List（`LPUSH` / `BRPOP` 构建任务） |
| 流水线配置 | 本地文件系统 YAML（每个流水线一个文件） |
| 前端 | Vue 3 + Vite + Arco Design Vue |
| 构建能力 | git / shell / docker CLI（执行器节点需自行安装） |

## 架构

```
┌────────────┐   REST/JSON    ┌──────────────────────────────┐
│  Vue +     │ ─────────────► │  Gin API                     │
│  Arco      │                │  ├─ 流水线 CRUD ──► YAML 文件 │
└────────────┘ ◄── 日志轮询 ── │  ├─ 触发运行 ─────► MySQL     │
                               │  └─ 入队 ─────────► Redis     │
                               └──────────────┬───────────────┘
                                              │ BRPOP
                               ┌──────────────▼───────────────┐
                               │  Executor Worker 池           │
                               │  checkout → stages → docker  │
                               │  状态写 MySQL，日志写本地文件  │
                               └──────────────────────────────┘
```

运行流程：

1. 前端触发运行，后端在 MySQL 写入一条 `pending` 运行记录，并把 run id `LPUSH` 到 Redis 队列；
2. Executor worker `BRPOP` 取出任务，进入 `running`，克隆代码（浅克隆）、逐个执行阶段；
3. 每个阶段的状态、退出码、耗时写入 MySQL；命令输出实时追加到 `data/logs/<run-id>.log`；
4. 前端通过 `GET /api/runs/:id/logs?offset=N` 增量拉取日志（返回新 offset 与运行状态），并轮询运行详情更新阶段状态；
5. 结束后运行状态为 `success` / `failed` / `canceled`，成功后默认清理工作目录（失败保留便于排查）。

## 目录结构

```
jinline/
├── backend/                     # Go 后端
│   ├── main.go                  # 入口：配置、MySQL/Redis、worker、HTTP
│   ├── configs/config.yaml      # 服务端配置
│   └── internal/
│       ├── api/                 # Gin 路由与 handler
│       ├── config/              # 配置加载（支持环境变量覆盖）
│       ├── executor/            # 任务消费与流水线执行
│       ├── model/               # PipelineRun / RunStage
│       ├── pipeline/            # 流水线 YAML 定义与校验
│       ├── queue/               # Redis 队列
│       └── store/               # MySQL / YAML 文件存储
├── frontend/                    # Vue 3 + Arco Design 前端
│   └── src/
│       ├── api/                 # REST 客户端与类型
│       ├── components/          # StatusTag / LogViewer
│       └── views/               # 流水线列表、编辑、详情、运行记录、运行详情
├── examples/                    # 示例流水线 YAML
├── docker-compose.yml           # 本地 MySQL + Redis
└── README.md
```

## 快速开始

### 1. 启动 MySQL 与 Redis

```bash
docker compose up -d
```

默认 MySQL：`root/123456`，数据库 `jinline`；Redis：`127.0.0.1:6379`。
如果你使用已有实例，修改 `backend/configs/config.yaml` 或通过环境变量覆盖：

| 环境变量 | 说明 |
| --- | --- |
| `JINLINE_ADDR` | 服务监听地址，默认 `:8080` |
| `JINLINE_STATIC_DIR` | 前端静态目录 |
| `JINLINE_MYSQL_DSN` | MySQL DSN |
| `JINLINE_REDIS_ADDR` / `JINLINE_REDIS_PASSWORD` / `JINLINE_REDIS_DB` | Redis 连接 |
| `JINLINE_QUEUE_KEY` | 队列 key，默认 `jinline:queue:build` |
| `JINLINE_WORKER_CONCURRENCY` | 并发构建数，默认 `2` |

> MySQL 中需要提前存在 `jinline` 库（docker-compose 已自动创建）：
> `CREATE DATABASE IF NOT EXISTS jinline DEFAULT CHARACTER SET utf8mb4;`
> 表结构由后端启动时自动迁移。

### 2. 启动后端

```bash
cd backend
go run . -c configs/config.yaml
```

服务默认监听 `http://127.0.0.1:8080`，健康检查：`GET /api/health`。

### 3. 启动前端（开发模式）

```bash
cd frontend
npm install
npm run dev
```

访问 `http://127.0.0.1:5173`，Vite 会把 `/api` 代理到后端 8080 端口。

### 4. 生产部署（单端口）

```bash
cd frontend && npm run build          # 产出 frontend/dist
cd ../backend && go build -o jinline .
./jinline -c configs/config.yaml      # 若 dist 存在，后端直接托管前端
```

## 构建部署产物

`scripts/build-dist.sh` 会构建前端、交叉编译 Linux 二进制，并组装出可直接部署的 `dist/` 目录（二进制、`web/`、配置模板、systemd 单元、安装脚本、MySQL 建表脚本、示例与部署文档）：

```bash
bash scripts/build-dist.sh                 # 默认 linux/amd64
GOARCH=arm64 bash scripts/build-dist.sh    # ARM64
VERSION=1.0.0 bash scripts/build-dist.sh   # 指定版本号
```

部署到 AlmaLinux 9.5 的完整步骤（MySQL、Redis、systemd、防火墙、SELinux、Docker、升级回滚）见 [docs/deploy-almalinux-9.5.md](docs/deploy-almalinux-9.5.md)。

## 流水线配置

配置以 YAML 文件保存在 `backend/data/pipelines/<name>.yaml`（相对启动目录），也可在 Web 界面中新建/编辑。

```yaml
apiVersion: v1
name: demo-go-build            # 唯一，字母/数字/-/_，最长 64
description: 示例流水线
git:
  url: https://github.com/example/repo.git
  branch: main                 # 可省略，默认仓库默认分支
  username: ""                 # 私有仓库可选
  password: ""
workdir: .                     # 构建目录，相对仓库根目录
env:                           # 全局环境变量
  CGO_ENABLED: "0"
stages:
  - name: build                # 命令阶段：在 shell 中依次执行
    commands:
      - go build ./...
    env:
      GOOS: linux

  - name: image                # 镜像阶段：docker build（push: true 时再 push）
    image: registry.example.com/team/app:${RUN_ID}
    dockerfile: Dockerfile
    context: .
    push: false
```

说明：

- 命令阶段在检出目录（`workdir`）下通过系统 shell 执行（Windows 为 `cmd /C`，其他平台为 `sh -c`）；
- 镜像阶段要求执行器节点已安装 Docker，`dockerfile`/`context` 相对 `workdir`；
- 执行时注入环境变量：`GIT_URL`、`GIT_BRANCH`、`GIT_COMMIT`、`PIPELINE_NAME`、`RUN_ID`、`WORKSPACE`；
- 全局 `env` 会被单个阶段内的同名 `env` 覆盖；
- 示例见 [`examples/`](examples/)。

## API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/health` | 健康检查 |
| GET | `/api/pipelines` | 流水线列表（含最近一次运行） |
| POST | `/api/pipelines` | 新建，body: `{"yaml": "..."}` 或原始 YAML 文本 |
| GET | `/api/pipelines/:name` | 详情 + 最近 20 次运行；`?raw=1` 返回原始 YAML |
| PUT | `/api/pipelines/:name` | 更新（允许改名） |
| DELETE | `/api/pipelines/:name` | 删除配置（历史运行保留） |
| POST | `/api/pipelines/:name/run` | 触发运行，body 可选 `{"branch": "dev"}` |
| GET | `/api/runs?pipeline=&page=&page_size=` | 运行记录分页 |
| GET | `/api/runs/:id` | 运行详情（含阶段） |
| GET | `/api/runs/:id/logs?offset=N` | 增量日志；响应头 `X-Log-Offset`、`X-Run-Status` |
| POST | `/api/runs/:id/cancel` | 取消排队中/运行中的任务 |

运行状态：`pending` → `running` → `success` / `failed` / `canceled`。

## 设计说明与限制

- 任务队列基于 Redis List，worker 与 API 在同一进程内启动，可通过 `worker.concurrency` 调整并发；
- 执行器直接调用宿主机的 `git`、shell、`docker` CLI，属于“本机执行”模型；如需隔离，可将 Executor 部署到独立节点并复用同一 MySQL/Redis；
- 取消操作通过 context 终止当前命令，但不会强制结束命令派生的全部子进程（如 docker 构建的远端进程）；
- YAML 中 `git.password` 不会通过 API 返回，但会以明文存储在本地配置文件中，注意文件权限；
- 日志按 run id 保存在本地 `data/logs/`，未做轮转与清理，长期使用建议配合定时清理策略。
