# Emma CSI Driver - Quick Installation Guide

This guide will help you install the Emma CSI Driver in your Kubernetes cluster in just a few minutes.

## Prerequisites

Before you begin, ensure you have:

✅ **Kubernetes cluster** (version 1.20 or higher)  
✅ **Worker nodes running as Emma.ms VMs** in the same Emma project  
✅ **kubectl** installed and configured to access your cluster  
✅ **Emma.ms account** with API access

## Installation Steps

### Step 1: Get Emma API Credentials

1. Log in to the [Emma Portal](https://portal.emma.ms)
2. Navigate to **Settings** → **Service Applications**
3. Click **Create Service Application**
4. Set access level to **"Manage"**
5. Copy the **Client ID** and **Client Secret**

### Step 2: Create Kubernetes Secret

Replace `YOUR_CLIENT_ID` and `YOUR_CLIENT_SECRET` with your actual credentials:

```bash
kubectl create secret generic emma-api-credentials \
  --from-literal=client-id=YOUR_CLIENT_ID \
  --from-literal=client-secret=YOUR_CLIENT_SECRET \
  --namespace=kube-system
```

### Step 3: Deploy the CSI Driver

Choose one of the following methods:

#### Option A: Using Kustomize (Recommended)

```bash
kubectl apply -k deploy/
```

#### Option B: Using kubectl directly

```bash
kubectl apply -f deploy/csidriver.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/configmap.yaml
kubectl apply -f deploy/controller.yaml
kubectl apply -f deploy/node.yaml
kubectl apply -f deploy/storageclass.yaml
```

#### Option C: Using a single command

```bash
kubectl apply -f https://raw.githubusercontent.com/YOUR_ORG/emma-csi-driver/main/deploy/
```

### Step 4: Verify Installation

Check that all components are running:

```bash
# Check controller (should show 1/1 READY)
kubectl get pods -n kube-system -l app=emma-csi-controller

# Check node plugins (should show one pod per worker node, all READY)
kubectl get pods -n kube-system -l app=emma-csi-node

# Verify CSI driver is registered
kubectl get csidrivers csi.emma.ms
```

Expected output:
```
NAME           ATTACHREQUIRED   PODINFOONMOUNT   STORAGECAPACITY   TOKENREQUESTS   REQUIRESREPUBLISH   MODES        AGE
csi.emma.ms    true             false            false             <unset>         false               Persistent   1m
```

### Step 5: Test with a Sample PVC

Create a test PVC to verify everything works:

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

Check the PVC status:

```bash
kubectl get pvc test-pvc
```

The PVC should transition to `Bound` status within 1-2 minutes:
```
NAME       STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
test-pvc   Bound    pvc-12345678-1234-1234-1234-123456789012   10Gi       RWO            emma-ssd       30s
```

### Step 6: Test with a Pod

Create a pod that uses the volume:

```bash
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: test-pvc
EOF
```

Verify the pod is running:

```bash
kubectl get pod test-pod
kubectl exec test-pod -- df -h /data
```

### Step 7: Clean Up Test Resources

```bash
kubectl delete pod test-pod
kubectl delete pvc test-pvc
```

## What's Next?

✅ **Configure StorageClasses**: See `deploy/storageclass.yaml` for examples  
✅ **Set up monitoring**: Prometheus metrics available on port 8080  
✅ **Review security**: Check `deploy/README.md` for security best practices  
✅ **Read documentation**: See `docs/` for detailed guides

## Available StorageClasses

After installation, you'll have these StorageClasses available:

| StorageClass | Type | Use Case | Reclaim Policy |
|--------------|------|----------|----------------|
| `emma-ssd` (default) | SSD | General purpose, databases | Delete |
| `emma-ssd-plus` | SSD Plus | High IOPS workloads | Delete |
| `emma-hdd` | HDD | Backups, archives | Delete |
| `emma-ssd-retain` | SSD | Data that must persist | Retain |
| `emma-ssd-xfs` | SSD | Large files, high throughput | Delete |

## Common Issues

### PVC Stuck in Pending

**Check controller logs:**
```bash
kubectl logs -n kube-system -l app=emma-csi-controller -c emma-csi-controller
```

**Common causes:**
- Invalid Emma API credentials
- Incorrect data center ID
- Network connectivity issues

### Pod Can't Mount Volume

**Check node plugin logs:**
```bash
kubectl logs -n kube-system -l app=emma-csi-node -c emma-csi-node
```

**Common causes:**
- Node is not an Emma.ms VM
- Missing filesystem tools (mkfs.ext4, mkfs.xfs)
- Volume already attached to another node

### Need Help?

- 📖 **Full documentation**: See `deploy/README.md`
- 🔧 **Troubleshooting guide**: See `docs/TROUBLESHOOTING.md`
- 📊 **API reference**: See `docs/API_REFERENCE.md`
- 🐛 **Report issues**: GitHub Issues

## Uninstallation

To remove the driver:

```bash
# 1. Delete all PVCs first
kubectl delete pvc --all

# 2. Remove the driver
kubectl delete -k deploy/

# 3. Delete the secret
kubectl delete secret emma-api-credentials -n kube-system
```

## Configuration Options

You can customize the driver by editing `deploy/configmap.yaml`:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: emma-csi-config
  namespace: kube-system
data:
  # Emma API endpoint
  api-url: "https://api.emma.ms/external"
  
  # Default data center (can be overridden per StorageClass)
  default-datacenter-id: "aws-eu-west-2"
  
  # Default volume type
  default-volume-type: "ssd"
  
  # Log level: debug, info, warn, error
  log-level: "info"
  
  # Enable JSON logging
  json-logging: "false"
```

After editing, restart the controller and node pods:

```bash
kubectl rollout restart statefulset emma-csi-controller -n kube-system
kubectl rollout restart daemonset emma-csi-node -n kube-system
```

---

**🎉 Congratulations!** You've successfully installed the Emma CSI Driver. Your Kubernetes cluster can now dynamically provision Emma.ms volumes.
