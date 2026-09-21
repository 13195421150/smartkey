# Smartkey CDK

可自托管的 CDK 兑换网站源码，包含 Vue 3 用户端与管理后台、Go API、SQLite 存储、测试和 Docker 部署配置。

本仓库不包含任何生产 API 密钥、后台密码、数据库、真实卡密、客户 Session、证书或服务器配置。部署者需要创建自己的管理员，并填写自己的卡台 API。

## 功能

- Session 单笔和批量兑换
- 卡密进度、失败详情与订阅状态查询
- 卡台 CDK 发码、同步、停用、对账与 Webhook
- 卡池诊断、选卡优先级和失败归因
- 北京时间展示与未完成订单恢复
- `PLUS-`、`Pro5X-`、`Pro20X-` 套餐前缀；原 `ZC-` 码仍兼容

## 部署

```bash
bash scripts/init-env.sh
# 编辑 .env，填写 CDK_DOMAIN
docker compose up -d --build
```

首次打开 `https://你的域名/ops/setup`，使用 `.env` 中的 `SETUP_BOOTSTRAP_TOKEN` 创建管理员；随后在 `/ops/integration` 配置自己的卡台 API。

完整说明见 [DEPLOYMENT.md](DEPLOYMENT.md)，套餐前缀见 [docs/site-cdk-prefixes.md](docs/site-cdk-prefixes.md)。

## 本地验证

```bash
cd frontend
npm ci
npm test
npm run typecheck:portal
npm run build

cd ../backend
go test ./...
```

SQLite 驱动使用 CGO，构建 Go 后端时需安装 C 编译器并设置 `CGO_ENABLED=1`。

## 来源

本项目基于 `zovocard/zovo_card_cdk_auto` 定制。第三方依赖遵循各自许可证；上游根目录没有提供可供本仓库重新授权的独立许可证，因此本仓库不额外声明 MIT/GPL 等许可证。
