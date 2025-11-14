# Emma CSI Driver Installation Guide

This guide provides step-by-step instructions for installing and configuring the Emma CSI Driver in your Kubernetes cluster.

## Prerequisites

Before installing the Emma CSI Driver, ensure you have the following:

### Kubernetes Cluster Requirements

- **Kubernetes Version**: 1.20 or higher
- **Container Runtime**: Any CSI-compatible runtime (containerd, CRI-O, Docker)
- **Worker Nodes**: Must be Emma.ms virtual machines within the same Emma.ms project
- **Cluster Access**: kubectl configured with cluster-admin privileges

### Emma.ms Requirements

- **Emma.ms Account**: Active account with API access
- **Service Application**: Created in Emma.ms with the following:
  - Access level: **Manage** (required for volume operations)
  - Client ID and Client Secret credentials
- **Project**: Worker nodes must belong to the same Emma.ms project as the service application

### Network Requirements

- Worker nodes must have outbound HTTPS access to `https://api.emma.ms`
- If using a proxy, ensure it's configured for the container runtime
- Firewall rules must allow gRPC communication between Kubernetes components and CSI driver

### Required Tools

- `kubectl` (version matching your cluster)
- `git` (for cloning the repository)
- Text editor for configuration files

## Installation Steps

### Step 1: Clone the Repository

```bash
git clone https://github.com/your-org/emma-csi-driver.git
cd emma-csi-driver
```

### Step 2: Create Namespace (Optional)

The driver can be installed in any namespace. The default is `kube-system`:

```bash
# If using a custom namespace
kubectl create namespace emma-csi-driver
```

For the rest of this guide, we'll use `kube-system`.

### Step 3: Configure Emma API Credentials

Create a Secret containing your Emma.ms API credentials:

```bash
kubectl create secret generic emma-api-credentials \
  --namespace=kube-system \
  --from-literal=client-id='your-client-id' \
  --from-literal=client-secret='your-client-secret'
```

**Important**: Replace `your-client-id` and `your-client-secret` with your actual Emma.ms service application credentials.

Alternatively, you can create the secret from a file:

```bash
# Create credentials file
cat > credentials.env <<EOF
client-id=your-client-id
client-secret=your-client-secret
EOF

# Create secret from file
kubectl create secret generic emma-api-credentials \
  --namespace=kube-system \
  --from-env-file=credentials.env

# Remove credentials file
rm credentials.env
```

### Step 4: Configure Driver Settings

Edit the ConfigMap to customize driver settings:

```bash
kubectl apply -f deploy/configmap.yaml
```

The default configuration is:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: emma-csi-driver-config
  namespace: kube-system
data:
  emma-api-url: "https://api.emma.ms/external"
  default-datacenter-id: ""  # Leave empty to use first available
  log-level: "info"
```

**Configuration Options**:

- `emma-api-url`: Emma API base URL (default: `https://api.emma.ms/external`)
- `default-datacenter-id`: Default datacenter for volume creation (e.g., `aws-eu-west-2`)
- `log-level`: Logging verbosity - `debug`, `info`, `warn`, or `error` (default: `info`)

### Step 5: Deploy RBAC Resources

Create the necessary ServiceAccounts, ClusterRoles, and ClusterRoleBindings:

```bash
kubectl apply -f deploy/rbac.yaml
```

This creates:
- `emma-csi-controller-sa` ServiceAccount for the controller
- `emma-csi-node-sa` ServiceAccount for node plugins
- ClusterRoles with required permissions
- ClusterRoleBindings

### Step 6: Deploy CSIDriver Resource

Register the CSI driver with Kubernetes:

```bash
kubectl apply -f deploy/csidriver.yaml
```

### Step 7: Deploy Controller

Deploy the CSI controller as a StatefulSet:

```bash
kubectl apply -f deploy/controller.yaml
```

Verify the controller is running:

```bash
kubectl get pods -n kube-system -l app=emma-csi-controller
```

Expected output:
```
NAME                    READY   STATUS    RESTARTS   AGE
emma-csi-controller-0   2/2     Running   0          30s
```

### Step 8: Deploy Node Plugin

Deploy the CSI node plugin as a DaemonSet:

```bash
kubectl apply -f deploy/node.yaml
```

Verify node plugins are running on all nodes:

```bash
kubectl get pods -n kube-system -l app=emma-csi-node
```

Expected output (one pod per node):
```
NAME                  READY   STATUS    RESTARTS   AGE
emma-csi-node-abc12   2/2     Running   0          30s
emma-csi-node-def34   2/2     Running   0          30s
emma-csi-node-ghi56   2/2     Running   0          30s
```

### Step 9: Create StorageClass

Create one or more StorageClasses for different volume types:

```bash
kubectl apply -f deploy/storageclass.yaml
```

This creates example StorageClasses:
- `emma-ssd`: SSD volumes
- `emma-ssd-plus`: High-performance SSD volumes
- `emma-hdd`: HDD volumes

### Step 10: Verify Installation

Check that all components are healthy:

```bash
# Check controller
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver

# Check node plugin (replace with actual pod name)
kubectl logs -n kube-system emma-csi-node-abc12 -c emma-csi-driver

# List available StorageClasses
kubectl get storageclass
```

Create a test PVC to verify functionality:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: test-pvc
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: emma-ssd
  resources:
    requests:
      storage: 10Gi
EOF
```

Check PVC status:

```bash
kubectl get pvc test-pvc
```

The PVC should be in `Pending` state until a pod uses it (due to `WaitForFirstConsumer` binding mode).

Clean up test PVC:

```bash
kubectl delete pvc test-pvc
```

## Configuration Options

### StorageClass Parameters

When creating StorageClasses, you can customize the following parameters:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: my-custom-storage
provisioner: csi.emma.ms
parameters:
  # Volume type: ssd, ssd-plus, or hdd
  type: ssd
  
  # Emma datacenter ID (optional)
  # If not specified, uses default from ConfigMap
  dataCenterId: aws-eu-west-2
  
  # Filesystem type: ext4 or xfs
  fsType: ext4

# Volume binding mode
volumeBindingMode: WaitForFirstConsumer  # Recommended

# Allow volume expansion
allowVolumeExpansion: true

# Reclaim policy: Delete or Retain
reclaimPolicy: Delete
```

**Parameter Details**:

- **type**: Volume performance tier
  - `ssd`: Standard SSD (balanced performance)
  - `ssd-plus`: High-performance SSD (premium)
  - `hdd`: Standard HDD (cost-effective)

- **dataCenterId**: Emma datacenter identifier
  - Format: `{provider}-{region}` (e.g., `aws-eu-west-2`, `gcp-us-central1`)
  - If omitted, uses `default-datacenter-id` from ConfigMap
  - Must match the datacenter of your worker nodes

- **fsType**: Filesystem format
  - `ext4`: Default, widely compatible
  - `xfs`: Better performance for large files

- **volumeBindingMode**:
  - `WaitForFirstConsumer`: Recommended - delays volume creation until pod is scheduled
  - `Immediate`: Creates volume immediately when PVC is created

- **allowVolumeExpansion**: Enable/disable volume resizing
  - `true`: Allows expanding PVC size (recommended)
  - `false`: Prevents volume expansion

- **reclaimPolicy**: What happens to volume when PVC is deleted
  - `Delete`: Automatically deletes Emma volume (recommended)
  - `Retain`: Keeps Emma volume for manual cleanup

### Controller Configuration

The controller deployment can be customized by editing `deploy/controller.yaml`:

**Resource Limits**:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

**Replica Count**:
```yaml
replicas: 1  # Only 1 replica supported (StatefulSet)
```

**Log Level** (via environment variable):
```yaml
env:
  - name: LOG_LEVEL
    value: "info"  # debug, info, warn, error
```

### Node Plugin Configuration

The node plugin deployment can be customized by editing `deploy/node.yaml`:

**Resource Limits**:
```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 200m
    memory: 256Mi
```

**Host Path Mounts** (required for device access):
```yaml
volumeMounts:
  - name: plugin-dir
    mountPath: /csi
  - name: pods-mount-dir
    mountPath: /var/lib/kubelet/pods
    mountPropagation: Bidirectional
  - name: device-dir
    mountPath: /dev
```

## Upgrading

To upgrade the Emma CSI Driver to a new version:

1. **Backup current configuration**:
```bash
kubectl get configmap emma-csi-driver-config -n kube-system -o yaml > configmap-backup.yaml
kubectl get secret emma-api-credentials -n kube-system -o yaml > secret-backup.yaml
```

2. **Update deployment manifests**:
```bash
git pull origin main
# Or download new manifests
```

3. **Apply updated manifests**:
```bash
kubectl apply -f deploy/
```

4. **Verify upgrade**:
```bash
kubectl rollout status statefulset/emma-csi-controller -n kube-system
kubectl rollout status daemonset/emma-csi-node -n kube-system
```

**Note**: Existing volumes and PVCs are not affected during upgrades.

## Uninstalling

To completely remove the Emma CSI Driver:

1. **Delete all PVCs using Emma StorageClasses**:
```bash
kubectl get pvc --all-namespaces -o json | \
  jq -r '.items[] | select(.spec.storageClassName | startswith("emma-")) | "\(.metadata.namespace) \(.metadata.name)"' | \
  xargs -n2 sh -c 'kubectl delete pvc -n $0 $1'
```

2. **Delete driver components**:
```bash
kubectl delete -f deploy/node.yaml
kubectl delete -f deploy/controller.yaml
kubectl delete -f deploy/csidriver.yaml
kubectl delete -f deploy/rbac.yaml
kubectl delete -f deploy/storageclass.yaml
kubectl delete -f deploy/configmap.yaml
kubectl delete secret emma-api-credentials -n kube-system
```

3. **Verify cleanup**:
```bash
kubectl get pods -n kube-system -l app.kubernetes.io/name=emma-csi-driver
```

## Troubleshooting Installation

### Controller Pod Not Starting

**Symptoms**: Controller pod in `CrashLoopBackOff` or `Error` state

**Check logs**:
```bash
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver
```

**Common causes**:
- Invalid Emma API credentials
- Network connectivity issues
- Missing ConfigMap or Secret

### Node Plugin Not Starting

**Symptoms**: Node plugin pods not running on some nodes

**Check DaemonSet status**:
```bash
kubectl describe daemonset emma-csi-node -n kube-system
```

**Common causes**:
- Node taints preventing scheduling
- Insufficient node resources
- Missing host path directories

### Authentication Failures

**Symptoms**: Logs show "401 Unauthorized" errors

**Solution**:
1. Verify credentials are correct:
```bash
kubectl get secret emma-api-credentials -n kube-system -o jsonpath='{.data.client-id}' | base64 -d
```

2. Recreate secret with correct credentials:
```bash
kubectl delete secret emma-api-credentials -n kube-system
kubectl create secret generic emma-api-credentials \
  --namespace=kube-system \
  --from-literal=client-id='correct-client-id' \
  --from-literal=client-secret='correct-client-secret'
```

3. Restart controller:
```bash
kubectl rollout restart statefulset/emma-csi-controller -n kube-system
```

### PVC Stuck in Pending

**Symptoms**: PVC remains in `Pending` state

**Check events**:
```bash
kubectl describe pvc <pvc-name>
```

**Common causes**:
- No pod using the PVC (with `WaitForFirstConsumer` mode)
- Invalid StorageClass parameters
- Insufficient quota in Emma account
- Datacenter mismatch

For more troubleshooting guidance, see [TROUBLESHOOTING.md](TROUBLESHOOTING.md).

## Next Steps

After successful installation:

1. Read the [User Guide](USER_GUIDE.md) for usage examples
2. Configure monitoring and alerts (see [Monitoring Guide](MONITORING.md))
3. Review [Best Practices](BEST_PRACTICES.md) for production deployments
4. Set up backup policies for critical volumes

## Support

For issues and questions:
- GitHub Issues: https://github.com/your-org/emma-csi-driver/issues
- Emma.ms Support: https://emma.ms/support
- Documentation: https://docs.emma.ms
