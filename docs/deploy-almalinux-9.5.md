# jinline 部署指南（AlmaLinux 9.5）

本文档描述如何将 jinline 部署到 AlmaLinux 9.5（x86_64）。部署形态为单机：jinline 服务、MySQL、Redis 运行在同一台服务器上；流水线中的编译命令与 Docker 构建执行在该服务器（执行器节点）上。

## 1. 部署架构

| 组件 | 作用 | 默认端口 |
| --- | --- | --- |
| jinline | Gin API + 前端静态资源 + 执行器 worker | 8080 |
| MySQL 8.0 | 流水线运行与阶段状态 | 3306 |
| Redis | 构建任务队列 | 6379 |
| Git / 构建工具链 / Docker | 流水线执行时由服务调用 | - |

服务通过 systemd 管理，进程以专用用户 `jinline` 运行，数据目录为 `/var/lib/jinline`。

## 2. 构建产物说明

在开发机或构建机上执行：

```bash
bash scripts/build-dist.sh            # 默认 linux/amd64
GOARCH=arm64 bash scripts/build-dist.sh   # ARM64 服务器
VERSION=1.0.0 bash scripts/build-dist.sh   # 指定版本号
```

脚本会完成前端构建、后端交叉编译（`CGO_ENABLED=0`），并组装出 `dist/`：

```
dist/
├── jinline                 # Linux 可执行文件（amd64，静态链接）
├── VERSION                 # 版本、commit、构建时间、目标平台
├── configs/
│   └── config.yaml         # 生产配置模板（需修改密码）
├── web/                    # 前端静态资源
├── deploy/
│   ├── install.sh          # 安装脚本
│   ├── jinline.service     # systemd 单元
│   └── schema.sql          # MySQL 建库建表脚本（可选）
├── examples/               # 示例流水线 YAML
└── docs/
    └── deploy-almalinux-9.5.md
```

将 `dist/` 打包上传到服务器：

```bash
tar -czf jinline-dist.tar.gz -C dist .
scp jinline-dist.tar.gz user@server:/tmp/
```

## 3. 系统准备

以下命令均以 root 或 sudo 执行。

```bash
# 更新系统并安装基础工具
dnf update -y
dnf install -y tar curl vim git

# 设置时区（按需）
timedatectl set-timezone Asia/Shanghai

# 确认系统信息
cat /etc/almalinux-release
uname -m
```

## 4. 安装与配置 MySQL 8

```bash
dnf install -y mysql-server
systemctl enable --now mysqld

# 安全初始化（设置 root 密码、移除匿名用户等）
mysql_secure_installation
```

创建数据库与专用账号（密码请自行替换，并与后续配置保持一致）：

```bash
mysql -uroot -p <<'SQL'
CREATE DATABASE IF NOT EXISTS jinline
  CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS 'jinline'@'127.0.0.1' IDENTIFIED BY 'ChangeMe_Db_123';
GRANT ALL PRIVILEGES ON jinline.* TO 'jinline'@'127.0.0.1';
FLUSH PRIVILEGES;
SQL
```

验证连接（表结构由 jinline 启动时自动创建，此处只需能连上）：

```bash
mysql -ujinline -p'ChangeMe_Db_123' -h127.0.0.1 -e "SELECT 1;"
```

### 可选：使用建表脚本

后端首次启动时会通过 GORM AutoMigrate 自动建表，通常无需手工执行。如需 DBA 预先建表、审计或迁移，可执行产物中的 `deploy/schema.sql`：

```bash
mysql -uroot -p < /tmp/jinline-dist/deploy/schema.sql
```

脚本包含 `CREATE DATABASE IF NOT EXISTS jinline` 以及 `pipeline_runs`、`run_stages` 两张表的 DDL（含索引与外键 `fk_pipeline_runs_stages`），可重复执行。安装完成后脚本也会保留在 `/opt/jinline/deploy/schema.sql`。

## 5. 安装与配置 Redis

```bash
dnf install -y redis
```

编辑 `/etc/redis/redis.conf`，至少确认以下两项：

```conf
bind 127.0.0.1 -::1
requirepass ChangeMe_Redis_123
```

```bash
systemctl enable --now redis
redis-cli -a 'ChangeMe_Redis_123' ping     # 期望输出 PONG
```

> Redis 只监听本机即可。若需要修改内存上限，可设置 `maxmemory` 与 `maxmemory-policy noeviction`，避免队列数据被淘汰。

## 6. 安装 jinline

### 方式一：使用安装脚本（推荐）

```bash
tar -xzf jinline-dist.tar.gz -C /tmp/jinline-dist
cd /tmp/jinline-dist
sudo ./deploy/install.sh
```

脚本会创建 `jinline` 系统用户、`/opt/jinline` 安装目录与 `/var/lib/jinline` 数据目录，复制程序、前端静态文件、示例、文档和 MySQL 建表脚本，安装 systemd 单元并设置开机自启。已存在的配置文件不会被覆盖。

### 方式二：手动安装

```bash
sudo useradd --system --home-dir /var/lib/jinline --shell /sbin/nologin jinline
sudo install -d -o jinline -g jinline -m 0750 /var/lib/jinline
sudo install -d -m 0755 /opt/jinline

sudo cp -a /tmp/jinline-dist/. /opt/jinline/
sudo install -m 0755 /tmp/jinline-dist/jinline /opt/jinline/jinline
sudo install -m 0640 -o root -g jinline \
  /tmp/jinline-dist/configs/config.yaml /opt/jinline/configs/config.yaml
sudo install -m 0644 /tmp/jinline-dist/deploy/jinline.service /etc/systemd/system/jinline.service
```

### 修改配置

编辑 `/opt/jinline/configs/config.yaml`，必改项：

```yaml
mysql:
  dsn: "jinline:ChangeMe_Db_123@tcp(127.0.0.1:3306)/jinline?charset=utf8mb4&parseTime=True&loc=Local"
redis:
  password: "ChangeMe_Redis_123"
```

其余配置见附录 A。配置文件权限保持 `root:jinline 0640`，不要放宽为所有人可读。

## 7. 启动与验证

```bash
systemctl daemon-reload          # 手动安装时执行；install.sh 已执行
systemctl enable --now jinline
systemctl status jinline --no-pager
journalctl -u jinline -f         # 查看启动日志
```

健康检查：

```bash
curl -s http://127.0.0.1:8080/api/health
# 期望：{"status":"ok","version":"<由构建脚本注入的版本>"}
```

浏览器访问 `http://<服务器IP>:8080`，用 `examples/demo-go-build.yaml` 的内容新建流水线并触发一次运行，确认：

1. 运行状态从“排队中”变为“运行中”，最终变为“成功”；
2. 阶段列表出现 `checkout`、`build`；
3. 日志页能实时输出 `go version`、`go build` 等内容；
4. 若使用默认示例的 `golang/example`，服务器需要能访问 GitHub，且已安装 Go（见第 9 节）。

## 8. 防火墙与 SELinux

```bash
# 放行 8080（如只在反向代理后使用，可不对外开放）
firewall-cmd --permanent --add-port=8080/tcp
firewall-cmd --reload
firewall-cmd --list-ports
```

SELinux 在 AlmaLinux 9 默认启用（`getenforce` 输出 `Enforcing`）。jinline 由 systemd 以非受限服务域启动，读取 `/opt/jinline`、写入 `/var/lib/jinline` 通常无需额外策略。若怀疑被拦截：

```bash
ausearch -m avc -ts recent
```

临时排查可将 `setenforce 0`，确认后务必改回 `setenforce 1`，并根据审计日志添加策略，而不是长期关闭 SELinux。

## 9. 流水线执行依赖（构建工具链）

流水线中的 `commands` 由执行器在宿主机 shell 中直接运行，因此**目标服务器需要安装对应语言/工具的编译环境**。例如：

```bash
# Go 项目（AlmaLinux 自带的 golang 版本可能较旧）
dnf install -y golang
go version

# 如需较新版本，可使用官方发行包
curl -LO https://go.dev/dl/go1.23.6.linux-amd64.tar.gz
rm -rf /usr/local/go
tar -C /usr/local -xzf go1.23.6.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' > /etc/profile.d/go.sh
source /etc/profile.d/go.sh

# Node.js 20（AlmaLinux 9 module）
dnf module install -y nodejs:20
node -v && npm -v
```

> 执行器使用 `sh -c` 运行命令，环境变量取自 systemd 进程环境 + 流水线 `env` + 阶段 `env`。systemd 服务不会加载 `/etc/profile.d`，如果构建命令依赖自定义 PATH，请在流水线 YAML 的 `env.PATH` 中显式指定，或修改 systemd 单元的 `Environment=`。

私有仓库凭据：

- HTTPS：在流水线 YAML 中配置 `git.username` / `git.password`（密码不会通过 API 返回，但会明文保存在 `/var/lib/jinline/pipelines/*.yaml`，注意文件权限）；
- SSH：为 `jinline` 用户生成密钥并配置到代码平台：

```bash
sudo install -d -o jinline -g jinline -m 0700 /var/lib/jinline/.ssh
sudo -u jinline ssh-keygen -t ed25519 -N '' -f /var/lib/jinline/.ssh/id_ed25519
# 将 /var/lib/jinline/.ssh/id_ed25519.pub 配置到 Git 平台
# 首次连接需要确认主机指纹：
sudo -u jinline git ls-remote git@github.com:org/repo.git
```

## 10. Docker 支持（可选）

镜像构建阶段（YAML 中配置 `image`）需要宿主机安装 Docker，并让 `jinline` 用户能访问 Docker socket。

```bash
dnf install -y dnf-plugins-core
dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin
systemctl enable --now docker

usermod -aG docker jinline
systemctl restart jinline

# 验证 jinline 用户可用
sudo -u jinline docker run --rm hello-world
```

> 注意：加入 `docker` 组等价于拥有 root 权限。仅应在可信团队使用本系统；更严格的环境可改用 rootless Podman（`dnf install podman podman-docker`），但其与 Docker 的行为差异需自行验证。

## 11. 反向代理与 HTTPS（可选）

如需通过 80/443 暴露服务，可使用 Nginx：

```bash
dnf install -y nginx
setsebool -P httpd_can_network_connect 1
systemctl enable --now nginx
```

```nginx
server {
    listen 80;
    server_name ci.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_read_timeout 300s;
    }
}
```

前端日志使用 1 秒轮询而非 WebSocket，普通反向代理配置即可。jinline 自身没有登录鉴权，暴露到公网前请在反向代理层增加认证，或仅允许内网访问。

## 12. 升级与回滚

升级前先备份（见第 13 节）。

```bash
systemctl stop jinline

# 备份当前二进制
cp /opt/jinline/jinline /opt/jinline/jinline.bak.$(date +%F)

# 解压新产物并覆盖程序与前端（保留 configs/ 与 /var/lib/jinline）
cd /tmp/jinline-dist-new
install -m 0755 jinline /opt/jinline/jinline
rm -rf /opt/jinline/web
cp -a web /opt/jinline/web
cp -a docs/. /opt/jinline/docs/
install -m 0644 deploy/jinline.service /etc/systemd/system/jinline.service

systemctl daemon-reload
systemctl start jinline
curl -s http://127.0.0.1:8080/api/health
```

表结构变更由启动时的 GORM AutoMigrate 自动完成（只做新增，不删除列）。若升级失败：

```bash
systemctl stop jinline
cp /opt/jinline/jinline.bak.<日期> /opt/jinline/jinline
systemctl start jinline
```

如新版本已写入不兼容数据，需要用备份的数据库恢复。

## 13. 备份

需要备份两类数据：

```bash
# MySQL（运行记录、阶段状态）
mysqldump -uroot -p --single-transaction --routines jinline > /backup/jinline-db-$(date +%F).sql

# 流水线 YAML 配置（以及需要时保留日志）
tar -czf /backup/jinline-data-$(date +%F).tar.gz \
  -C /var/lib/jinline pipelines logs
```

`/var/lib/jinline/workspace` 是临时构建目录，无需备份。建议通过 cron 每日备份，并定期清理 `logs` 目录。

## 14. 故障排查

| 现象 | 排查方向 |
| --- | --- |
| 服务反复重启，日志显示“等待 MySQL 就绪” | DSN 是否正确、库是否创建、账号是否授权、`systemctl status mysqld`、`ss -lntp \| grep 3306` |
| 日志显示“等待 Redis 就绪 … NOAUTH” | `redis.password` 与 `requirepass` 不一致，或 Redis 未启动 |
| 触发运行后一直“排队中” | worker 是否启动（日志中有 `executor started with N workers`）、Redis 队列 key 是否一致、worker 并发是否被占满 |
| `git clone` 失败（exit 128） | 服务器到 Git 平台网络/代理、私有仓库凭据、`sudo -u jinline git clone <url> /tmp/t` 手动验证 |
| 构建失败“未找到 go/npm” | 宿主机未安装工具链，或 PATH 未传给 systemd（见第 9 节） |
| docker 阶段失败“permission denied /var/run/docker.sock” | 未安装 Docker、未把 jinline 加入 docker 组、加入后未重启服务 |
| 8080 端口占用 | `ss -lntp \| grep 8080`，修改 `server.addr` 或停止占用进程 |
| 页面 404 / 白屏 | `/opt/jinline/web/index.html` 是否存在、`static_dir` 是否正确 |
| 中文乱码 | 数据库使用 utf8mb4，DSN 保留 `charset=utf8mb4` |
| 磁盘被日志占满 | 清理 `/var/lib/jinline/logs`，日志未做自动轮转 |

常用命令：

```bash
journalctl -u jinline -n 200 --no-pager
journalctl -u jinline -f
ss -lntp | grep -E '8080|3306|6379'
systemctl status mysqld redis jinline --no-pager
```

## 15. 安全建议

1. 修改配置模板中的数据库与 Redis 密码，配置文件保持 `0640` 且属组为 `jinline`；
2. MySQL 只监听本机（默认）或通过防火墙限制来源；Redis 设置密码并 `bind 127.0.0.1`；
3. 8080 默认不对公网开放；需要公网访问时使用 Nginx + HTTPS + 认证；
4. 流水线 YAML 中的命令拥有 `jinline` 用户权限，仅允许可信人员编辑流水线；
5. 为 `jinline` 用户配置最小权限，非必要不加入 `docker` 组（等价 root）；
6. 定期备份 MySQL 与 `/var/lib/jinline/pipelines`，并监控磁盘与日志大小；
7. 关注升级：升级前备份，升级后验证 `/api/health` 与一次完整流水线运行。

## 附录 A：配置项说明

| 配置项 | 说明 | 默认值 |
| --- | --- | --- |
| `server.addr` | HTTP 监听地址 | `:8080` |
| `server.static_dir` | 前端静态目录，相对 systemd WorkingDirectory | `./web` |
| `mysql.dsn` | MySQL DSN，需含 `charset=utf8mb4&parseTime=True` | 无 |
| `redis.addr` / `redis.password` / `redis.db` | Redis 连接 | `127.0.0.1:6379` / 空 / 0 |
| `redis.queue_key` | 构建任务队列 key | `jinline:queue:build` |
| `workspace.root` | 代码检出与构建目录 | `/var/lib/jinline/workspace` |
| `workspace.pipelines_dir` | 流水线 YAML 目录 | `/var/lib/jinline/pipelines` |
| `workspace.logs_dir` | 运行日志目录 | `/var/lib/jinline/logs` |
| `workspace.cleanup_on_success` | 成功后清理工作目录 | `true` |
| `worker.concurrency` | 并发构建数 | `2` |

也可用环境变量覆盖（systemd 单元中设置 `Environment=`）：

| 环境变量 | 对应配置 |
| --- | --- |
| `JINLINE_ADDR` | `server.addr` |
| `JINLINE_STATIC_DIR` | `server.static_dir` |
| `JINLINE_MYSQL_DSN` | `mysql.dsn` |
| `JINLINE_REDIS_ADDR` / `JINLINE_REDIS_PASSWORD` / `JINLINE_REDIS_DB` | Redis 连接 |
| `JINLINE_QUEUE_KEY` | `redis.queue_key` |
| `JINLINE_WORKER_CONCURRENCY` | `worker.concurrency` |

## 附录 B：systemd 单元

安装后的单元位于 `/etc/systemd/system/jinline.service`：

```ini
[Unit]
Description=jinline CI/CD pipeline service
After=network-online.target mysqld.service redis.service
Wants=network-online.target

[Service]
Type=simple
User=jinline
Group=jinline
WorkingDirectory=/opt/jinline
Environment=HOME=/var/lib/jinline
ExecStart=/opt/jinline/jinline -c /opt/jinline/configs/config.yaml
Restart=on-failure
RestartSec=5s
LimitNOFILE=65536
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/jinline

[Install]
WantedBy=multi-user.target
```
