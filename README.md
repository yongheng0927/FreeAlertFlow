# 烽火台（Fenghuo）

**烽火台（Fenghuo）** 是一个自托管的告警转发中台：接收 Alertmanager 的 webhook 告警，按路由规则分组匹配，经按渠道类型编写的 Go template 消息模板渲染成消息 payload 后投递到 IM 机器人（飞书 / 钉钉 / 企业微信）。

项目名取自中国古代的烽火台：边疆遇警，昼燔燧、夜举烽，一座座烽火台接力传递，警讯片刻直达中枢。两千年前的边防告警网络，做的事和今天的告警管道一模一样——**采集、路由、送达**。烽火台的追求也和它一样：快、准、不丢。

## 功能特性

- 支持飞书自定义机器人、钉钉自定义机器人、企业微信群机器人；飞书/钉钉完整支持签名校验（加签）
- 自定义消息模板：内容即渠道消息 payload 的 Go template（按渠道类型编写），支持在线预览
- Web 管理界面：通知渠道、路由规则、告警记录、失败投递一键重发
- 多用户 RBAC（admin / editor / viewer），支持飞书 OAuth 登录
- 前端静态资源内嵌（`go:embed`），仅支持 Docker / Helm 部署：docker compose 一键起本地环境，Helm chart 部署到 K8s

## 快速开始

```bash
cp docker-compose.example.yaml docker-compose.yaml   # 按需修改密钥等配置
docker compose up -d --build
```

访问 http://localhost:8080 ，使用 compose 中 `FENGHUO_ADMIN_USER` / `FENGHUO_ADMIN_PASSWORD` 配置的初始管理员登录。

## 配置

全部配置经环境变量注入（`FENGHUO_` 前缀），完整环境变量表见 [docs/REQUIREMENTS.md](docs/REQUIREMENTS.md)。本地开发也可以使用 `config.yaml`（已被 gitignore，不提供示例文件），配置键与环境变量的映射规则为点号换下划线（如 `server.http_addr` ↔ `FENGHUO_SERVER_HTTP_ADDR`）；优先级：环境变量 > config.yaml > 默认值。

## 文档

- [需求文档](docs/REQUIREMENTS.md)
- [数据库设计](docs/DATABASE_DESIGN.md)

## License

见 [LICENSE](LICENSE)。
