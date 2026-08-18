# Fenghuo（烽火台）Helm Chart

在无内部 registry 的离线/内网 K8s 集群上部署 Fenghuo。

## 前置条件

- Kubernetes >= 1.25，Helm 3
- **外部 PostgreSQL 数据库**：chart 不再内置数据库，需提前建好库和用户（默认 `fenghuo`/`fenghuo`）
- 集群节点运行时为 containerd，镜像通过手动导入分发（`imagePullPolicy: IfNotPresent`）

## 1. 构建并分发镜像

```bash
docker build -t fenghuo:latest .
docker save fenghuo:latest -o fenghuo.tar
```

把 tar 包拷到**每个节点**（app 可能被调度到任意 worker），导入 containerd：

```bash
# 在每个节点上执行（注意 k8s.io 命名空间，否则 kubelet 看不到）
sudo ctr -n k8s.io images import fenghuo.tar
```

## 2. 密钥管理（两种方式二选一）

应用需要两个密钥：`FENGHUO_JWT_SECRET`、数据库密码。

### 方式 A：引用已有 Secret（推荐生产使用）

预先创建 Secret，key 名约定如下：

```bash
kubectl create secret generic fenghuo-secrets -n fenghuo \
  --from-literal=jwt-secret=<随机串> \
  --from-literal=database-password=<数据库密码>
```

安装时引用它（此时 `secrets.jwtSecret` / `database.password` 无需设置）：

```bash
helm install fenghuo ./deploy/helm/fenghuo \
  -n fenghuo --create-namespace \
  --set database.host=<数据库地址> \
  --set secrets.existingSecret=fenghuo-secrets
```

### 方式 B：由 chart 生成 Secret

```bash
helm install fenghuo ./deploy/helm/fenghuo \
  -n fenghuo --create-namespace \
  --set database.host=<数据库地址> \
  --set database.password=<数据库密码> \
  --set secrets.jwtSecret=<随机串>
```

> `database.host` 为必填；方式 B 下 `secrets.jwtSecret` / `database.password`
> 均必填，未提供时 helm 会直接报错。chart 不提供默认弱密钥。

## 3. 验证

```bash
helm list -n fenghuo
kubectl get pods -n fenghuo
kubectl port-forward -n fenghuo svc/fenghuo 8080:8080   # 访问 http://localhost:8080
```

初始管理员通过首次访问 Web 界面按引导设置（FR-5.1），无需在 values 中预置账号。

## 4. 可选配置

### 通过 Higress Ingress 暴露

```bash
helm upgrade fenghuo ./deploy/helm/fenghuo -n fenghuo \
  --set database.host=<数据库地址> \
  --set secrets.existingSecret=fenghuo-secrets \
  --set ingress.enabled=true --set ingress.host=fenghuo.local \
  --set rootUrl=http://fenghuo.local/
```

### 多副本

`replicaCount` 大于 1 时需要共享的外部数据库——本 chart 只支持外部数据库，该前提已天然
满足，直接 `--set replicaCount=2` 即可。

### 最小 values 示例

```yaml
database:
  host: "postgres.example.internal"
  password: "s3cret"
secrets:
  jwtSecret: "change-me"
```

### 其他 values

见 [values.yaml](./values.yaml)，所有应用配置与 `config.example.yaml` 中的 `FENGHUO_*`
环境变量一一对应。

## 卸载

```bash
helm uninstall fenghuo -n fenghuo
# chart 不创建任何 PVC；方式 A 的外部 Secret 也不会被删除
```
