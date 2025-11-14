# Emma CSI Driver Deployment

This directory contains Kubernetes manifests for deploying the Emma CSI Driver.

## Prerequisites

- Kubernetes cluster version 1.20 or higher
- Worker nodes must be Emma.ms VMs in the same project
- Emma.ms Service Application with "Manage" access level
- `kubectl` configured to access your cluster

## Quick Start

### 1. Create Emma API Credentials Secret

First, obtain your Emma.ms API credentials:
1. Log in to https://portal.emma.ms
2. Navigate to Settings > Service Applications
3. Create a new Service Application with "Manage" access level
4. Copy the Client ID and Client Secret

Create the secret:

```bash
kubectl create secret generic emma-api-credentials \
  --from-literal=client-id=YOUR_CLIENT_ID \
  --from-literal=client-secret=YOUR_CLIENT_SECRET \
  --namespace=kube-system
```

### 2. Configure Driver Settings (Optional)

Edit `configmap.yaml` to customize:
- Emma API URL
- Default data center ID
- Default volume type
- Log level
- Other driver settings

### 3. Deploy the Driver

Using kustomize (recommended):

```bash
kubectl apply -k deploy/
```

Or deploy individual manifests:

```bash
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/controller.yaml
kubectl apply -f deploy/node.yaml
kubectl apply -f deploy/storageclass.yaml
```

### 4. Verify Installation

Check that the driver pods are running:

```bash
# Check controller pod
kubectl get pods -n kube-system -l app=emma-csi-controller

# Check node pods (one per worker node)
kubectl get pods -n kube-system -l app=emma-csi-node

# Check CSI driver registration
kubectl get csidrivers
```

### 5. Create a Test PVC

```bash
kubectl apply -f - <<EOF
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

## Manifest Files

- **csidriver.yaml**: CSIDriver resource defining driver capabilities
- **rbac.yaml**: ServiceAccounts, ClusterRoles, and ClusterRoleBindings
- **configmap.yaml**: Driver configuration settings
- **secret.yaml**: Template for Emma API credentials (must be customized)
- **controller.yaml**: StatefulSet for the CSI controller component
- **node.yaml**: DaemonSet for the CSI node plugin component
- **storageclass.yaml**: Example StorageClasses and usage examples
- **kustomization.yaml**: Kustomize configuration for easy deployment

## StorageClass Options

The driver supports multiple StorageClasses for different use cases:

### emma-ssd (default)
High-performance SSD storage for databases and latency-sensitive applications.

### emma-ssd-plus
Premium SSD storage with enhanced IOPS for demanding workloads.

### emma-hdd
Cost-effective HDD storage for backups and archival data.

### emma-ssd-retain
SSD storage with Retain reclaim policy (volumes not deleted with PVC).

### emma-ssd-xfs
SSD storage with XFS filesystem for large files and high throughput.

## StorageClass Parameters

```yaml
parameters:
  type: ssd              # Volume type: ssd, ssd-plus, hdd
  dataCenterId: aws-eu-west-2  # Emma data center ID
  fsType: ext4           # Filesystem: ext4, xfs
```

## Volume Expansion

All StorageClasses support volume expansion. To resize a volume:

```bash
kubectl patch pvc test-pvc -p '{"spec":{"resources":{"requests":{"storage":"20Gi"}}}}'
```

The filesystem will be automatically expanded by the node plugin.

## Monitoring

The controller exposes Prometheus metrics on port 8080:

```bash
kubectl port-forward -n kube-system svc/emma-csi-controller-metrics 8080:8080
curl http://localhost:8080/metrics
```

## Troubleshooting

### Check controller logs
```bash
kubectl logs -n kube-system -l app=emma-csi-controller -c emma-csi-controller
```

### Check node plugin logs
```bash
kubectl logs -n kube-system -l app=emma-csi-node -c emma-csi-node
```

### Check CSI sidecar logs
```bash
# Provisioner
kubectl logs -n kube-system -l app=emma-csi-controller -c csi-provisioner

# Attacher
kubectl logs -n kube-system -l app=emma-csi-controller -c csi-attacher

# Resizer
kubectl logs -n kube-system -l app=emma-csi-controller -c csi-resizer
```

### Common Issues

**PVC stuck in Pending**
- Check that the StorageClass exists and references the correct provisioner
- Verify Emma API credentials are correct
- Check controller logs for errors

**Volume not attaching**
- Ensure nodes are Emma.ms VMs in the same project
- Verify the data center ID matches your node location
- Check that the volume limit (16 per node) is not exceeded

**Mount failures**
- Check node plugin logs
- Verify the filesystem type is supported (ext4, xfs)
- Ensure the node has required tools (mkfs.ext4, mkfs.xfs)

## Uninstallation

To remove the driver:

```bash
# Delete all PVCs using Emma volumes first
kubectl delete pvc --all

# Delete the driver
kubectl delete -k deploy/

# Or delete individual manifests
kubectl delete -f deploy/
```

## Security Notes

- Never commit `secret.yaml` with real credentials to version control
- Use a secret management solution (Sealed Secrets, External Secrets) in production
- Rotate API credentials regularly
- Ensure Service Application has only "Manage" access level
- Review and adjust RBAC permissions as needed

## Support

For issues and questions:
- Check the troubleshooting section above
- Review driver logs for error messages
- Consult Emma.ms API documentation: https://docs.emma.ms
