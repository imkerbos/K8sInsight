# GKE 集群接入指南

K8sInsight 使用标准 Kubernetes kubeconfig 连接集群。GKE 默认依赖 `gke-gcloud-auth-plugin` exec 插件进行认证，该插件要求运行环境安装 gcloud SDK 并持有有效凭据，不适合服务端长期运行的场景。

本文档介绍如何为 GKE 集群生成基于 ServiceAccount 静态 Token 的 kubeconfig，以便接入 K8sInsight。

## 前置条件

- 本地已安装 `gcloud` CLI 和 `kubectl`
- 本地已安装 `gke-gcloud-auth-plugin`（用于初始连接）
- 拥有目标 GKE 集群的管理员权限
- K8sInsight 服务端网络可达 GKE 集群 API Server

## 操作步骤

### 1. 连接到目标 GKE 集群

```bash
gcloud container clusters get-credentials <CLUSTER_NAME> \
  --zone <ZONE> \
  --project <PROJECT_ID>
```

### 2. 创建专用 ServiceAccount

```bash
# 创建命名空间
kubectl create namespace k8s-insight || true

# 创建 ServiceAccount
kubectl create serviceaccount k8s-insight-sa -n k8s-insight
```

### 3. 绑定权限

根据实际需求选择合适的 ClusterRole：

```bash
# 只读权限（推荐，满足监控需求）
kubectl create clusterrolebinding k8s-insight-binding \
  --clusterrole=view \
  --serviceaccount=k8s-insight:k8s-insight-sa
```

> 如需更高权限，可将 `view` 替换为 `cluster-admin` 或自定义 ClusterRole。

### 4. 创建永久 Token

Kubernetes 1.24+ 不再自动为 ServiceAccount 创建永久 Token，需要手动创建：

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Secret
metadata:
  name: k8s-insight-sa-token
  namespace: k8s-insight
  annotations:
    kubernetes.io/service-account.name: k8s-insight-sa
type: kubernetes.io/service-account-token
EOF
```

等待几秒后，获取 Token 和 CA 证书：

```bash
TOKEN=$(kubectl get secret k8s-insight-sa-token -n k8s-insight \
  -o jsonpath='{.data.token}' | base64 -d)

CA_CERT=$(kubectl get secret k8s-insight-sa-token -n k8s-insight \
  -o jsonpath='{.data.ca\.crt}')
```

### 5. 获取集群 API Server 地址

```bash
ENDPOINT=$(gcloud container clusters describe <CLUSTER_NAME> \
  --zone <ZONE> \
  --project <PROJECT_ID> \
  --format="value(endpoint)")

echo "API Server: https://${ENDPOINT}"
```

### 6. 生成 kubeconfig

将上述信息填入以下模板：

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: <CA_CERT>
    server: https://<ENDPOINT>
  name: gke-cluster
contexts:
- context:
    cluster: gke-cluster
    user: k8s-insight-sa
  name: gke-context
current-context: gke-context
users:
- name: k8s-insight-sa
  user:
    token: <TOKEN>
```

或使用脚本一键生成：

```bash
cat <<EOF
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority-data: ${CA_CERT}
    server: https://${ENDPOINT}
  name: gke-cluster
contexts:
- context:
    cluster: gke-cluster
    user: k8s-insight-sa
  name: gke-context
current-context: gke-context
users:
- name: k8s-insight-sa
  user:
    token: ${TOKEN}
EOF
```

### 7. 接入 K8sInsight

1. 登录 K8sInsight 管理界面
2. 进入「集群管理」页面
3. 点击「添加集群」
4. 输入集群名称，将生成的 kubeconfig YAML 粘贴到输入框
5. 保存后系统会自动测试连接并启动监控

## Token 说明

通过 `kubernetes.io/service-account-token` 类型 Secret 生成的 Token **永不过期**，只要 Secret 和 ServiceAccount 存在就一直有效。

| Token 类型 | 有效期 | 适用场景 |
|------------|--------|----------|
| Secret 绑定 Token（本文方案） | 永久 | 服务端长期连接 |
| TokenRequest API（`kubectl create token`） | 默认 1 小时 | 临时调试 |
| Pod 挂载 Projected Token | 默认 1 小时，自动刷新 | Pod 内部使用 |

Token 失效条件：
- 删除对应的 Secret
- 删除对应的 ServiceAccount
- 删除 ClusterRoleBinding（Token 仍有效但无权限）

## GKE 网络配置

K8sInsight 服务端必须能访问 GKE 集群的 API Server。根据集群类型：

- **公共集群**：确保 K8sInsight 服务端的出口 IP 在 GKE [Authorized Networks](https://cloud.google.com/kubernetes-engine/docs/how-to/authorized-networks) 白名单中
- **私有集群**：需通过 VPN、Cloud Interconnect 或在同一 VPC 内部署 K8sInsight

## 安全建议

1. **最小权限原则**：优先使用 `view`（只读）ClusterRole，避免使用 `cluster-admin`
2. **定期轮换 Token**：建议每 90 天轮换一次，操作如下：
   ```bash
   # 删除旧 Secret（Token 立即失效）
   kubectl delete secret k8s-insight-sa-token -n k8s-insight
   # 重新创建（参考步骤 4）
   # 在 K8sInsight 中更新集群的 kubeconfig
   ```
3. **审计日志**：在 GKE 中启用 Admin Activity 审计日志，监控 ServiceAccount 的 API 调用
4. **独立 ServiceAccount**：每个 K8sInsight 实例使用独立的 ServiceAccount，便于追踪和撤销

## 故障排查

| 问题 | 可能原因 | 解决方法 |
|------|----------|----------|
| 连接超时 | 网络不通 | 检查防火墙和 Authorized Networks 配置 |
| 401 Unauthorized | Token 无效或已失效 | 检查 Secret 和 ServiceAccount 是否存在 |
| 403 Forbidden | 权限不足 | 检查 ClusterRoleBinding 是否正确 |
| x509 证书错误 | CA 证书不匹配 | 重新获取 CA 证书并更新 kubeconfig |
