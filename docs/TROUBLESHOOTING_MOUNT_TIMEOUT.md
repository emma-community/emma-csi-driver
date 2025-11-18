# Troubleshooting Mount Timeout Issues

## Problem: "context deadline exceeded" during volume mount

### Symptoms

```
Warning  FailedMount  kubelet  MountVolume.MountDevice failed for volume "pvc-xxx": 
         rpc error: code = DeadlineExceeded desc = context deadline exceeded
```

This error occurs when the CSI node plugin cannot mount the volume within Kubernetes' default timeout (120 seconds).

## Root Causes

### 1. Device Not Appearing on Node

**Most Common Cause**: The volume is attached to the VM at the Emma API level, but the device is not appearing in the operating system.

**Why this happens:**
- Cloud provider (AWS/GCP/Azure) takes time to make device visible
- udev rules not creating device symlinks
- Device naming pattern doesn't match what the driver expects
- VM needs time to scan for new devices

### 2. Device Discovery Taking Too Long

The CSI driver searches for the device using multiple patterns:
- `/dev/disk/by-id/virtio-{volumeID}` (KVM/legacy)
- `/dev/disk/by-id/google-{volumeID}` (GCP)
- `/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol*` (AWS)
- `/dev/disk/by-id/scsi-*` (Azure/GCP)

If the device appears with an unexpected name, discovery can timeout.

### 3. Volume Attachment Delays

Emma API attach operations can take time due to:
- VM in transitional state (starting, stopping, etc.)
- API rate limiting
- Cloud provider delays
- Network issues

## Diagnostic Steps

### Step 1: Run the Diagnostic Script

```bash
sudo ./scripts/diagnose-mount-timeout.sh <pod-name> [namespace]
```

Example:
```bash
sudo ./scripts/diagnose-mount-timeout.sh example-pod default
```

This will show:
- PVC and PV information
- Volume ID
- Node where pod is scheduled
- Recent logs from node plugin and controller
- Suggested commands to run on the node

### Step 2: Check Controller Logs

```bash
# Get controller pod name
CONTROLLER_POD=$(kubectl get pods -n kube-system -l app=emma-csi-controller -o jsonpath='{.items[0].metadata.name}')

# Check if volume was attached successfully
kubectl logs -n kube-system $CONTROLLER_POD -c emma-csi-controller | grep -i "attach"
```

Look for:
- ✅ `Volume X attach to VM Y initiated successfully`
- ❌ `failed to attach volume`
- ⚠️ `VM not ready for attach` (indicates retries)

### Step 3: Check Node Plugin Logs

```bash
# Get the node name where pod is scheduled
NODE_NAME=$(kubectl get pod <pod-name> -o jsonpath='{.spec.nodeName}')

# Get node plugin pod on that node
NODE_PLUGIN=$(kubectl get pods -n kube-system -l app=emma-csi-node \
  --field-selector spec.nodeName=$NODE_NAME -o jsonpath='{.items[0].metadata.name}')

# Check logs
kubectl logs -n kube-system $NODE_PLUGIN -c emma-csi-node --tail=100
```

Look for:
- ✅ `Found device /dev/xxx for volume YYY`
- ❌ `timeout waiting for device`
- ⚠️ `Waiting for device to appear` (indicates device not found yet)

### Step 4: Inspect Devices on Node

SSH to the node and check:

```bash
# List all block devices
lsblk

# Check /dev/disk/by-id/ for your volume
ls -la /dev/disk/by-id/

# Check recent kernel messages
dmesg | tail -50

# Trigger udev rescan
udevadm trigger --subsystem-match=block
udevadm settle

# Check for specific device types
ls -la /dev/disk/by-id/virtio-*     # KVM/legacy
ls -la /dev/disk/by-id/nvme-*       # AWS
ls -la /dev/disk/by-id/google-*     # GCP
ls -la /dev/disk/by-id/scsi-*       # Azure/GCP
```

### Step 5: Check Volume Attachment Status

```bash
# List volume attachments
kubectl get volumeattachment

# Describe specific attachment
kubectl describe volumeattachment | grep -A 20 "pvc-<your-pvc-id>"
```

Look for:
- `Attached: true` - Volume is attached
- `Attached: false` - Volume attachment failed or pending

## Solutions

### Solution 1: Wait and Retry

Sometimes the device just needs more time to appear.

```bash
# Delete the pod to trigger a retry
kubectl delete pod <pod-name>

# The pod will be recreated and try mounting again
```

### Solution 2: Restart Node Plugin

If the node plugin is stuck or has stale state:

```bash
# Get node plugin pod
NODE_PLUGIN=$(kubectl get pods -n kube-system -l app=emma-csi-node \
  --field-selector spec.nodeName=<node-name> -o jsonpath='{.items[0].metadata.name}')

# Delete it (will be recreated by DaemonSet)
kubectl delete pod -n kube-system $NODE_PLUGIN

# Wait for it to restart
kubectl wait --for=condition=ready pod -n kube-system -l app=emma-csi-node \
  --field-selector spec.nodeName=<node-name> --timeout=60s
```

### Solution 3: Manually Trigger udev on Node

SSH to the node and force udev to rescan:

```bash
# Trigger udev for block devices
udevadm trigger --subsystem-match=block

# Wait for udev to settle
udevadm settle --timeout=30

# Check if device appeared
ls -la /dev/disk/by-id/
```

### Solution 4: Check Cloud Provider Compatibility

Ensure your node is running on the expected cloud provider:

```bash
# Check cloud provider metadata
curl -s http://169.254.169.254/latest/meta-data/instance-id  # AWS
curl -s -H "Metadata-Flavor: Google" http://metadata.google.internal/computeMetadata/v1/instance/id  # GCP
curl -s -H "Metadata:true" "http://169.254.169.254/metadata/instance?api-version=2021-02-01"  # Azure
```

### Solution 5: Increase Log Verbosity

To get more detailed logs:

```bash
# Edit node plugin DaemonSet
kubectl edit daemonset emma-csi-node -n kube-system

# Change log-level from "info" to "debug"
# Find: --log-level=info
# Replace with: --log-level=debug

# Restart node plugin pods
kubectl rollout restart daemonset emma-csi-node -n kube-system
```

### Solution 6: Check Volume Limits

Emma.ms has a limit of 16 volumes per VM:

```bash
# Count volumes attached to node
kubectl get volumeattachment -o json | \
  jq -r '.items[] | select(.spec.nodeName=="<node-name>") | .metadata.name' | wc -l
```

If at limit, detach unused volumes or use a different node.

## Prevention

### 1. Use WaitForFirstConsumer Binding Mode

This ensures volumes are created in the same datacenter as the pod:

```yaml
volumeBindingMode: WaitForFirstConsumer
```

### 2. Set Appropriate Timeouts

In your pod spec, you can increase the mount timeout:

```yaml
spec:
  containers:
  - name: app
    volumeMounts:
    - name: data
      mountPath: /data
  volumes:
  - name: data
    persistentVolumeClaim:
      claimName: my-pvc
  # Note: Kubernetes doesn't expose mount timeout directly
  # The timeout is controlled by kubelet flags
```

### 3. Pre-warm Nodes

If you know you'll need volumes, you can pre-attach them:

```bash
# Create PVC ahead of time
kubectl apply -f pvc.yaml

# Wait for it to be bound
kubectl wait --for=jsonpath='{.status.phase}'=Bound pvc/my-pvc --timeout=120s

# Then create the pod
kubectl apply -f pod.yaml
```

### 4. Monitor Volume Attachment Time

Set up monitoring for volume attachment duration:

```bash
# Check Prometheus metrics
curl http://<controller-pod-ip>:8080/metrics | grep emma_volume_attach
```

## Code Changes (v1.1.0+)

Recent improvements to reduce mount timeouts:

1. **Reduced device discovery timeout**: From 180s to 90s to stay within K8s limits
2. **Earlier alternative discovery**: Try NVMe/cloud provider patterns after 20s instead of waiting full timeout
3. **Faster udev triggers**: Trigger every 10s instead of only at the end
4. **Better logging**: More detailed logs to diagnose issues faster
5. **Parallel discovery**: Try multiple device patterns simultaneously

## Still Having Issues?

### Collect Full Diagnostics

```bash
# Run diagnostic script
sudo ./scripts/diagnose-mount-timeout.sh <pod-name> <namespace> > diagnostics.txt

# Collect all CSI logs
kubectl logs -n kube-system -l app=emma-csi-controller --all-containers > controller-logs.txt
kubectl logs -n kube-system -l app=emma-csi-node --all-containers > node-logs.txt

# Get cluster info
kubectl describe nodes > nodes.txt
kubectl get volumeattachment -o yaml > volumeattachments.yaml
kubectl get pv -o yaml > pvs.yaml
kubectl get pvc --all-namespaces -o yaml > pvcs.yaml
```

### Report Issue

Open a GitHub issue with:
1. Output from diagnostic script
2. Controller and node plugin logs
3. Cloud provider (AWS/GCP/Azure)
4. Emma datacenter ID
5. Node OS and kernel version
6. Steps to reproduce

## Related Documentation

- [Installation Guide](INSTALLATION.md)
- [User Guide](USER_GUIDE.md)
- [Architecture](../ARCHITECTURE.md)
- [Emma API Documentation](https://docs.emma.ms)
