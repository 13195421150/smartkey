# 部署说明

## 1. 准备条件

- 一台可访问卡台接口与依赖下载站点的服务器
- 已解析到服务器的域名
- Docker Engine 与 Docker Compose
- 80、443 端口可用，或自行接入现有反向代理

## 2. 初始化配置

```bash
bash scripts/init-env.sh
```

脚本会从 `.env.example` 生成权限为 600 的 `.env`，并创建随机 `JWT_SECRET` 和 `SETUP_BOOTSTRAP_TOKEN`。编辑 `.env`：

```dotenv
CDK_DOMAIN=cdk.example.com
```

不要把 `.env`、数据库、Session 或 API Key 提交到 Git。

## 3. 启动

```bash
docker compose config --quiet
docker compose up -d --build
docker compose ps
docker compose logs --tail=100 cdk
```

健康检查：

```bash
curl https://cdk.example.com/health
```

## 4. 创建管理员与接入卡台

1. 打开 `https://你的域名/ops/setup`。
2. 输入 `.env` 中的安装令牌，创建自己的管理员。
3. 登录 `/ops/login`，进入 `/ops/integration`。
4. 填写自己的卡台 Base URL 与 API Key；按需要设置 IP 白名单。
5. 在卡台为当前 API Key 配置 Webhook：`https://你的域名/api/v1/webhooks/cardplatform`。
6. 将卡台生成的 Webhook Secret 保存到本站后台。

本仓库没有通用初始账号或密码。安装完成后安装令牌不能再次创建管理员。

## 5. 套餐前缀

从卡台同步完整码后，本站按套餐显示并导出：

- Plus：`PLUS-`
- Pro5x：`Pro5X-`
- Pro20x：`Pro20X-`

原始 `ZC-` 码继续有效，新旧写法共用同一兑换状态；自定义前缀只能用于已适配的本站。

## 6. 数据与备份

Compose 卷 `cdk_data` 保存管理员、API 设置、卡密关系、订单记录和提交的敏感数据。停止服务后备份卷和 `.env`；不要把它们上传到源码仓库。

```bash
docker compose stop cdk
docker compose run --rm --no-deps -T --entrypoint tar cdk -C /app/data -czf - . > cdk-data-backup.tgz
docker compose start cdk
```

`docker compose down` 不会删除卷；不要追加 `-v`，除非明确要永久删除运行数据。

## 7. 原生运行

需要 Node.js 22、Go 1.26.x 和 C 编译器：

```bash
cd frontend && npm ci && npm run build
cd ../backend
CGO_ENABLED=1 go build -trimpath -o ../cdk-recharge ./cmd/server
```

设置 `DB_PATH`、`WEB_DIR`、`SERVER_HOST`、`SERVER_PORT`、`JWT_SECRET` 等环境变量后运行二进制，并使用现有 HTTPS 代理转发到后端监听端口。

## 8. 更新与安全

- 默认 `UPDATE_ENABLED=0`，避免未知上游更新覆盖定制代码。
- 更新前备份数据库和 `.env`，再重新构建容器。
- `/api/`、兑换结果和管理页面不应由 CDN 静态缓存。
- 不要把真实卡密、客户 Session、API Key、Webhook Secret 或数据库写入日志。
