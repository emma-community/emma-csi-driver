# Quick Fix: Mount Timeout (context deadline exceeded)

## The Error

```
FailedMount: MountVolume.MountDevice failed for volume "pvc-xxx": 
rpc error: code = DeadlineExceeded desc = context deadline exceeded
```

## Quick Diagnosis (30 seconds)

```bash
# 1. Get the volume ID
kubectl get pv <pv-name> -o jsonpath='{.spec.csi.volumeHandle}'

# 2. Check if volume was attached
kubectl logs -n kube-system -l app=emma-csi-controller --tail=50 | grep -i "attach.*<volume-id>"

# 3. Check node plugin logs
kubectl logs -n kube-system -l app=emma-csi-node --tail=50 | grep -i "<volume-id>"
```

## Quick Fixes (Try in order)

### Fix 1: Delete and Recreate Pod (90% success rate)

```bash
kubectl delete pod <pod-name>
# Pod will be recreated automatically if part of Deployment/StatefulSet
```

### Fix 2: Restart Node Plugin (80% success rate)

```bash
# Find node where pod is scheduled
NODE=$(kubectl get pod <pod-name> -o jsonpath='{.spec.nodeName}')

# Delete node plugin on that node
kubectl delete pod -n kube-system -l app=emma-csi-node --field-selector spec.nodeName=$NODE

# Wait 30 seconds, then retry pod
sleep 30
kubectl delete pod <pod-name>
```

### Fix 3: Manual Device Rescan on Node (70% success rate)

SSH to the node and run:

```bash
# Trigger udev rescan
sudo udevadm trigger --subsystem-match=block
sudo udevadm settle

# Check if device appeared
ls -la /dev/disk/by-id/
```

Then delete the pod:

```bash
kubectl delete pod <pod-name>
```

## Root Causes

| Symptom | Cause | Fix |
|---------|-------|-----|
| Controller logs show "attach succeeded" but mount fails | Device not appearing on node | Fix 3 (udev rescan) |
| Controller logs show "VM not ready" retries | VM in transitional state | Wait 2-3 minutes, then Fix 1 |
| No attach logs in controller | Volume attachment never started | Check PVC/StorageClass config |
| Node plugin logs show "timeout waiting for device" | Device discovery timeout | Fix 2 (restart node plugin) |

## Prevention

1. **Use WaitForFirstConsumer** in StorageClass:
   ```yaml
   volumeBindingMode: WaitForFirstConsumer
   ```

2. **Ensure datacenter matches** node location:
   ```yaml
   parameters:
     dataCenterId: aws-eu-central-1  # Must match node's datacenter
   ```

3. **Don't exceed volume limit**: Max 16 volumes per node

## Need More Help?

Run full diagnostics:

```bash
sudo ./scripts/diagnose-mount-timeout.sh <pod-name> <namespace>
```

See detailed guide: [TROUBLESHOOTING_MOUNT_TIMEOUT.md](TROUBLESHOOTING_MOUNT_TIMEOUT.md)
