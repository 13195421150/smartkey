#!/usr/bin/env bash
set -euo pipefail
cd -- "$(dirname -- "$0")/.."
if [[ -e .env ]]; then
  echo '.env 已存在，保留原文件。请直接编辑域名或配置。'
  exit 0
fi
command -v openssl >/dev/null || { echo '请先安装 openssl。'; exit 1; }
umask 077
cdk_jwt_value="$(openssl rand -hex 32)"
cdk_setup_value="$(openssl rand -hex 24)"
sed -e "s/^JWT_SECRET=$/JWT_SECRET=${cdk_jwt_value}/" \
    -e "s/^SETUP_BOOTSTRAP_TOKEN=$/SETUP_BOOTSTRAP_TOKEN=${cdk_setup_value}/" \
    .env.example > .env
chmod 600 .env
echo '已生成本机专属 .env。请填写 CDK_DOMAIN。'
echo '首次安装钥匙在 .env 的 SETUP_BOOTSTRAP_TOKEN 字段；请勿把 .env 发给别人。'
