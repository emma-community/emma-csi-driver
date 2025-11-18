# Emma CSI Driver Helm Chart

A Helm chart for deploying the Emma CSI Driver on Kubernetes.

## Prerequisites

- Kubernetes 1.20+
- Helm 3.0+
- Worker nodes must be Emma.ms VMs
- Emma.ms Service Application with "Manage" access level

## Installation

### Quick Start

```bash
# Add Helm repository (if published)
helm repo add emma https://charts.emma.ms
helm repo update

# Install with Emma credentials
helm install emma-csi-driver emma/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.clientId=YOUR_CLIENT_ID \
  --set emma.credentials.clientSecret=YOUR_CLIENT_SECRET
```

### Install from Local Chart

```bash
# Install from local directory
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.clientId=YOUR_CLIENT_ID \
  --set emma.credentials.clientSecret=YOUR_CLIENT_SECRET
```

### Install with Custom Values

```bash
# Create values file
cat > my-values.yaml <<EOF
emma:
  credentials:
    clientId: "your-client-id"
    clientSecret: "your-client-secret"
  defaultDatacenterId: "aws-eu-central-1"

controller:
  logLevel: debug
  resources:
    requests:
      memory: 256Mi

node:
  logLevel: debug
EOF

# Install with custom values
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --values my-values.yaml
```

## Configuration

### Emma API Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `emma.apiUrl` | Emma API URL | `https://api.emma.ms/external` |
| `emma.credentials.clientId` | Emma API Client ID | `""` (required) |
| `emma.credentials.clientSecret` | Emma API Client Secret | `""` (required) |
| `emma.credentials.existingSecret` | Use existing secret | `""` |
| `emma.defaultDatacenterId` | Default datacenter ID | `""` |

### Controller Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `controller.replicaCount` | Number of replicas | `1` |
| `controller.image.repository` | Controller image repository | `arsenh1995/ghaghaqoqoqo123` |
| `controller.image.tag` | Controller image tag | `csi-controller` |
| `controller.logLevel` | Log level (debug/info/warn/error) | `info` |
| `controller.resources` | Resource limits and requests | See values.yaml |

### Node Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `node.image.repository` | Node image repository | `arsenh1995/ghaghaqoqoqo123` |
| `node.image.tag` | Node image tag | `csi-node` |
| `node.logLevel` | Log level (debug/info/warn/error) | `info` |
| `node.resources` | Resource limits and requests | See values.yaml |
| `node.kubeletDir` | Kubelet directory | `/var/lib/kubelet` |

### Storage Classes

| Parameter | Description | Default |
|-----------|-------------|---------|
| `storageClasses.create` | Create storage classes | `true` |
| `storageClasses.default` | Default storage class name | `emma-ssd` |
| `storageClasses.classes` | List of storage class configurations | See values.yaml |

## Examples

### Basic Installation

```bash
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.clientId=abc123 \
  --set emma.credentials.clientSecret=secret456
```

### With Specific Datacenter

```bash
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.clientId=abc123 \
  --set emma.credentials.clientSecret=secret456 \
  --set emma.defaultDatacenterId=aws-eu-central-1
```

### With Debug Logging

```bash
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.clientId=abc123 \
  --set emma.credentials.clientSecret=secret456 \
  --set controller.logLevel=debug \
  --set node.logLevel=debug
```

### Using Existing Secret

```bash
# Create secret first
kubectl create secret generic emma-credentials \
  --from-literal=client-id=abc123 \
  --from-literal=client-secret=secret456 \
  --namespace=kube-system

# Install with existing secret
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.existingSecret=emma-credentials
```

### Custom Storage Classes

```yaml
# custom-storage.yaml
storageClasses:
  create: true
  default: my-ssd
  classes:
    - name: my-ssd
      default: true
      reclaimPolicy: Delete
      volumeBindingMode: WaitForFirstConsumer
      allowVolumeExpansion: true
      parameters:
        type: ssd
        fsType: ext4
      dataCenterId: aws-eu-central-1
    
    - name: my-hdd
      default: false
      reclaimPolicy: Delete
      volumeBindingMode: WaitForFirstConsumer
      allowVolumeExpansion: true
      parameters:
        type: hdd
        fsType: ext4
      dataCenterId: gcp-europe-west1
```

```bash
helm install emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --set emma.credentials.clientId=abc123 \
  --set emma.credentials.clientSecret=secret456 \
  --values custom-storage.yaml
```

## Upgrading

```bash
# Upgrade to new version
helm upgrade emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --reuse-values

# Upgrade with new values
helm upgrade emma-csi-driver ./charts/emma-csi-driver \
  --namespace kube-system \
  --values my-values.yaml
```

## Uninstalling

```bash
# Delete all PVCs first
kubectl delete pvc --all

# Uninstall the chart
helm uninstall emma-csi-driver --namespace kube-system
```

## Verification

```bash
# Check installation
helm list -n kube-system

# Check pods
kubectl get pods -n kube-system -l app.kubernetes.io/name=emma-csi-driver

# Check CSI driver
kubectl get csidrivers

# Check storage classes
kubectl get storageclass
```

## Troubleshooting

### Controller Pod Not Starting

```bash
# Check controller logs
kubectl logs -n kube-system -l app=emma-csi-controller -c emma-csi-controller

# Check events
kubectl describe pod -n kube-system -l app=emma-csi-controller
```

### Node Pod Not Starting

```bash
# Check node logs
kubectl logs -n kube-system -l app=emma-csi-node -c emma-csi-node

# Check DaemonSet
kubectl describe daemonset -n kube-system emma-csi-driver-node
```

### PVC Stuck in Pending

```bash
# Check PVC events
kubectl describe pvc <pvc-name>

# Check controller logs
kubectl logs -n kube-system -l app=emma-csi-controller -c emma-csi-controller | grep <pvc-name>
```

## Support

For issues and questions:
- GitHub: https://github.com/your-org/emma-csi-driver/issues
- Documentation: https://docs.emma.ms
- Emma Support: https://emma.ms/support
