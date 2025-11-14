# Emma CSI Driver Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the Emma CSI Driver.

## Table of Contents

- [General Troubleshooting Steps](#general-troubleshooting-steps)
- [Common Issues and Solutions](#common-issues-and-solutions)
- [Log Analysis](#log-analysis)
- [Metrics Interpretation](#metrics-interpretation)
- [Advanced Debugging](#advanced-debugging)

## General Troubleshooting Steps

When encountering issues with the Emma CSI Driver, follow these steps:

1. **Check component health**:
```bash
# Check controller status
kubectl get pods -n kube-system -l app=emma-csi-controller

# Check node plugin status
kubectl get pods -n kube-system -l app=emma-csi-node

# Check CSIDriver registration
kubectl get csidriver csi.emma.ms
```

2. **Review logs**:
```bash
# Controller logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver --tail=100

# Node plugin logs (replace pod name)
kubectl logs -n kube-system emma-csi-node-xxxxx -c emma-csi-driver --tail=100
```

3. **Check events**:
```bash
# PVC events
kubectl describe pvc <pvc-name>

# Pod events
kubectl describe pod <pod-name>

# Node events
kubectl get events --sort-by='.lastTimestamp' -n kube-system
```

4. **Verify configuration**:
```bash
# Check credentials
kubectl get secret emma-api-credentials -n kube-system

# Check ConfigMap
kubectl get configmap emma-csi-driver-config -n kube-system -o yaml

# Check StorageClass
kubectl get storageclass <storageclass-name> -o yaml
```

## Common Issues and Solutions

### Volume Provisioning Issues

#### PVC Stuck in Pending State

**Symptoms**:
- PVC remains in `Pending` status indefinitely
- No PersistentVolume created

**Diagnosis**:
```bash
kubectl describe pvc <pvc-name>
```

**Common Causes and Solutions**:

1. **WaitForFirstConsumer binding mode** (Expected behavior)
   - **Cause**: StorageClass uses `volumeBindingMode: WaitForFirstConsumer`
   - **Solution**: Create a pod that uses the PVC. Volume will be provisioned when pod is scheduled
   ```bash
   # Check binding mode
   kubectl get storageclass <class-name> -o jsonpath='{.volumeBindingMode}'
   ```

2. **Invalid StorageClass parameters**
   - **Cause**: Incorrect `type` or `dataCenterId` in StorageClass
   - **Solution**: Verify parameters against Emma API:
   ```bash
   # Check available datacenters
   curl -H "Authorization: Bearer <token>" \
     https://api.emma.ms/external/v1/data-centers
   
   # Check available volume configs
   curl -H "Authorization: Bearer <token>" \
     https://api.emma.ms/external/v1/volume-configs
   ```
   - Update StorageClass with valid parameters

3. **Emma API quota exceeded**
   - **Cause**: Account has reached volume or storage quota
   - **Solution**: Check Emma.ms dashboard for quota limits or contact support

4. **Authentication failure**
   - **Cause**: Invalid or expired API credentials
   - **Solution**: See [Authentication Issues](#authentication-issues)

5. **Datacenter mismatch**
   - **Cause**: Requested datacenter doesn't match node location
   - **Solution**: Ensure `dataCenterId` in StorageClass matches worker node datacenter

#### Volume Creation Fails

**Symptoms**:
- PVC shows `ProvisioningFailed` event
- Controller logs show volume creation errors

**Diagnosis**:
```bash
# Check controller logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep -i "createvolume"

# Check PVC events
kubectl describe pvc <pvc-name> | grep -A 10 Events
```

**Common Causes and Solutions**:

1. **Invalid volume size**
   - **Cause**: Requested size not supported by Emma volume configs
   - **Solution**: Check available sizes:
   ```bash
   # Sizes must match Emma volume configs (typically: 10, 20, 50, 100, 200, 500, 1000 GB)
   ```
   - Adjust PVC size to supported value

2. **Invalid volume type**
   - **Cause**: StorageClass `type` parameter invalid
   - **Solution**: Use valid types: `ssd`, `ssd-plus`, or `hdd`

3. **Network timeout**
   - **Cause**: Emma API unreachable or slow
   - **Solution**: Check network connectivity and retry

4. **Emma API error**
   - **Cause**: Emma platform issue
   - **Solution**: Check Emma.ms status page and retry

### Volume Attachment Issues

#### Volume Fails to Attach to Node

**Symptoms**:
- Pod stuck in `ContainerCreating` state
- Events show `FailedAttachVolume` or `FailedMount`

**Diagnosis**:
```bash
# Check pod events
kubectl describe pod <pod-name>

# Check controller logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep -i "controllerpublish"

# Check VolumeAttachment
kubectl get volumeattachment
kubectl describe volumeattachment <attachment-name>
```

**Common Causes and Solutions**:

1. **Node VM ID not found**
   - **Cause**: Node is not an Emma.ms VM or metadata unavailable
   - **Solution**: Verify nodes are Emma VMs in the same project:
   ```bash
   # Check node labels
   kubectl get nodes --show-labels
   ```

2. **Volume already attached elsewhere**
   - **Cause**: Volume attached to another node (ReadWriteOnce limitation)
   - **Solution**: Delete old pod or wait for detachment:
   ```bash
   # Check volume attachments
   kubectl get volumeattachment -o wide
   ```

3. **Attachment timeout**
   - **Cause**: Emma API slow to attach volume
   - **Solution**: Wait for operation to complete (may take 1-2 minutes)

4. **Maximum volumes per node exceeded**
   - **Cause**: Node has reached 16 volume limit
   - **Solution**: Reduce volumes on node or use different node

#### Volume Fails to Detach

**Symptoms**:
- PVC deletion hangs
- VolumeAttachment remains after pod deletion

**Diagnosis**:
```bash
# Check VolumeAttachment status
kubectl get volumeattachment

# Check controller logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep -i "controllerunpublish"
```

**Common Causes and Solutions**:

1. **Pod still using volume**
   - **Cause**: Pod not fully terminated
   - **Solution**: Force delete pod if necessary:
   ```bash
   kubectl delete pod <pod-name> --force --grace-period=0
   ```

2. **Detachment timeout**
   - **Cause**: Emma API slow to detach
   - **Solution**: Wait for retry (driver retries with exponential backoff)

3. **Volume stuck in ACTIVE state**
   - **Cause**: Emma platform issue
   - **Solution**: Manually detach via Emma.ms dashboard or API

### Volume Mounting Issues

#### Volume Fails to Mount on Node

**Symptoms**:
- Pod stuck in `ContainerCreating`
- Events show `FailedMount` or `MountVolume.SetUp failed`

**Diagnosis**:
```bash
# Check pod events
kubectl describe pod <pod-name>

# Check node plugin logs
kubectl logs -n kube-system <emma-csi-node-pod> -c emma-csi-driver | grep -i "nodestage\|nodepublish"
```

**Common Causes and Solutions**:

1. **Device not found**
   - **Cause**: Volume attached but device not visible on node
   - **Solution**: Wait for device to appear (may take 10-30 seconds after attachment)
   ```bash
   # SSH to node and check devices
   lsblk
   ```

2. **Filesystem formatting failed**
   - **Cause**: Invalid fsType or device issues
   - **Solution**: Check node plugin logs for formatting errors
   - Verify fsType is `ext4` or `xfs`

3. **Mount point already in use**
   - **Cause**: Stale mount from previous pod
   - **Solution**: Manually unmount on node:
   ```bash
   # SSH to node
   umount /var/lib/kubelet/pods/<pod-uid>/volumes/kubernetes.io~csi/<pv-name>/mount
   ```

4. **Insufficient permissions**
   - **Cause**: Node plugin lacks required privileges
   - **Solution**: Verify node plugin runs with `privileged: true` security context

### Volume Expansion Issues

#### Volume Expansion Fails

**Symptoms**:
- PVC size increase doesn't take effect
- Events show `VolumeResizeFailed`

**Diagnosis**:
```bash
# Check PVC status
kubectl describe pvc <pvc-name>

# Check controller logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep -i "expand"
```

**Common Causes and Solutions**:

1. **StorageClass doesn't allow expansion**
   - **Cause**: `allowVolumeExpansion: false` in StorageClass
   - **Solution**: Update StorageClass (doesn't affect existing PVCs):
   ```bash
   kubectl patch storageclass <class-name> -p '{"allowVolumeExpansion": true}'
   ```

2. **Size decrease attempted**
   - **Cause**: New size smaller than current size
   - **Solution**: Volume shrinking not supported. Only increases allowed.

3. **Invalid new size**
   - **Cause**: New size not in Emma volume configs
   - **Solution**: Use supported sizes (10, 20, 50, 100, 200, 500, 1000 GB)

4. **Filesystem resize failed**
   - **Cause**: Node plugin failed to expand filesystem
   - **Solution**: Check node plugin logs and verify filesystem type supports online resize

#### Filesystem Not Expanded After Volume Resize

**Symptoms**:
- PVC shows new size but pod sees old size
- `df -h` in pod shows old capacity

**Diagnosis**:
```bash
# Check PVC conditions
kubectl get pvc <pvc-name> -o jsonpath='{.status.conditions}'

# Check node plugin logs
kubectl logs -n kube-system <emma-csi-node-pod> -c emma-csi-driver | grep -i "nodeexpand"
```

**Solution**:
- Restart pod to trigger filesystem expansion:
```bash
kubectl delete pod <pod-name>
```

### Authentication Issues

#### API Authentication Failures

**Symptoms**:
- Controller logs show "401 Unauthorized" errors
- Operations fail with authentication errors

**Diagnosis**:
```bash
# Check controller logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep -i "auth\|401"

# Verify secret exists
kubectl get secret emma-api-credentials -n kube-system
```

**Solutions**:

1. **Invalid credentials**
   - Verify credentials in Emma.ms dashboard
   - Recreate secret with correct values:
   ```bash
   kubectl delete secret emma-api-credentials -n kube-system
   kubectl create secret generic emma-api-credentials \
     --namespace=kube-system \
     --from-literal=client-id='your-client-id' \
     --from-literal=client-secret='your-client-secret'
   ```
   - Restart controller:
   ```bash
   kubectl rollout restart statefulset/emma-csi-controller -n kube-system
   ```

2. **Token refresh failure**
   - Check controller logs for refresh errors
   - Verify Emma API is accessible
   - Restart controller to force new token issuance

3. **Service application disabled**
   - Check Emma.ms dashboard
   - Ensure service application is active with "Manage" access level

### Performance Issues

#### Slow Volume Operations

**Symptoms**:
- Volume provisioning takes several minutes
- Attachment/detachment operations timeout

**Diagnosis**:
```bash
# Check operation latencies in metrics
kubectl port-forward -n kube-system emma-csi-controller-0 8080:8080
curl http://localhost:8080/metrics | grep emma_csi_operation_duration
```

**Solutions**:

1. **Emma API latency**
   - Check Emma.ms status page
   - Consider using datacenter closer to your location

2. **Network issues**
   - Verify network connectivity to api.emma.ms
   - Check for proxy or firewall delays

3. **Rate limiting**
   - Check logs for 429 errors
   - Reduce concurrent operations
   - Contact Emma support for rate limit increase

#### High Resource Usage

**Symptoms**:
- Controller or node plugin using excessive CPU/memory
- Pods being OOMKilled

**Diagnosis**:
```bash
# Check resource usage
kubectl top pod -n kube-system -l app.kubernetes.io/name=emma-csi-driver

# Check resource limits
kubectl get pod -n kube-system emma-csi-controller-0 -o jsonpath='{.spec.containers[0].resources}'
```

**Solutions**:

1. **Increase resource limits**:
   ```bash
   # Edit controller deployment
   kubectl edit statefulset emma-csi-controller -n kube-system
   
   # Edit node plugin
   kubectl edit daemonset emma-csi-node -n kube-system
   ```

2. **Reduce log level**:
   ```bash
   # Update ConfigMap
   kubectl patch configmap emma-csi-driver-config -n kube-system \
     -p '{"data":{"log-level":"warn"}}'
   
   # Restart pods
   kubectl rollout restart statefulset/emma-csi-controller -n kube-system
   kubectl rollout restart daemonset/emma-csi-node -n kube-system
   ```

## Log Analysis

### Understanding Log Levels

The driver uses structured JSON logging with the following levels:

- **DEBUG**: Detailed traces, API request/response bodies
- **INFO**: Normal operations, lifecycle events
- **WARN**: Retryable errors, degraded performance
- **ERROR**: Operation failures, non-retryable errors

### Key Log Patterns

#### Successful Volume Creation

```json
{
  "timestamp": "2025-01-15T10:30:45Z",
  "level": "info",
  "component": "controller",
  "operation": "CreateVolume",
  "volumeName": "pvc-abc123",
  "sizeGB": 10,
  "type": "ssd",
  "message": "Creating volume via Emma API"
}
{
  "timestamp": "2025-01-15T10:30:47Z",
  "level": "info",
  "component": "controller",
  "operation": "CreateVolume",
  "volumeId": "12345",
  "status": "AVAILABLE",
  "message": "Volume created successfully",
  "duration_ms": 2500
}
```

#### Failed Volume Creation

```json
{
  "timestamp": "2025-01-15T10:30:45Z",
  "level": "error",
  "component": "controller",
  "operation": "CreateVolume",
  "volumeName": "pvc-abc123",
  "error": "invalid volume type: invalid-type",
  "message": "Volume creation failed"
}
```

#### Volume Attachment

```json
{
  "timestamp": "2025-01-15T10:31:00Z",
  "level": "info",
  "component": "controller",
  "operation": "ControllerPublishVolume",
  "volumeId": "12345",
  "nodeId": "67890",
  "message": "Attaching volume to node"
}
{
  "timestamp": "2025-01-15T10:31:15Z",
  "level": "info",
  "component": "controller",
  "operation": "ControllerPublishVolume",
  "volumeId": "12345",
  "devicePath": "/dev/vdb",
  "message": "Volume attached successfully",
  "duration_ms": 15000
}
```

#### API Retry

```json
{
  "timestamp": "2025-01-15T10:32:00Z",
  "level": "warn",
  "component": "emma-client",
  "operation": "AttachVolume",
  "volumeId": "12345",
  "httpStatus": 503,
  "attempt": 1,
  "maxRetries": 5,
  "message": "API request failed, retrying with backoff"
}
```

### Filtering Logs

```bash
# Show only errors
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep '"level":"error"'

# Show specific operation
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep '"operation":"CreateVolume"'

# Show API errors
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep '"httpStatus":[45]'

# Show slow operations (>5 seconds)
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver | grep '"duration_ms"' | awk -F'"duration_ms":' '{print $2}' | awk -F',' '{if($1>5000) print}'
```

### Enabling Debug Logging

For detailed troubleshooting, enable debug logging:

```bash
# Update ConfigMap
kubectl patch configmap emma-csi-driver-config -n kube-system \
  -p '{"data":{"log-level":"debug"}}'

# Restart controller
kubectl rollout restart statefulset/emma-csi-controller -n kube-system

# Restart node plugins
kubectl rollout restart daemonset/emma-csi-node -n kube-system
```

**Warning**: Debug logging generates large log volumes. Revert to `info` level after troubleshooting.

## Metrics Interpretation

### Accessing Metrics

The driver exposes Prometheus metrics on port 8080:

```bash
# Port forward to controller
kubectl port-forward -n kube-system emma-csi-controller-0 8080:8080

# Fetch metrics
curl http://localhost:8080/metrics
```

### Key Metrics

#### Operation Metrics

```
# Total operations by type and status
emma_csi_operations_total{operation="CreateVolume",status="success"} 42
emma_csi_operations_total{operation="CreateVolume",status="failure"} 2

# Operation latency (histogram)
emma_csi_operation_duration_seconds_bucket{operation="CreateVolume",le="1"} 10
emma_csi_operation_duration_seconds_bucket{operation="CreateVolume",le="5"} 35
emma_csi_operation_duration_seconds_bucket{operation="CreateVolume",le="10"} 42
emma_csi_operation_duration_seconds_sum{operation="CreateVolume"} 156.7
emma_csi_operation_duration_seconds_count{operation="CreateVolume"} 42
```

**Interpretation**:
- Success rate: `success / (success + failure)` = 42/44 = 95.5%
- Average latency: `sum / count` = 156.7/42 = 3.7 seconds
- P95 latency: Most operations complete within 5 seconds

#### API Request Metrics

```
# API requests by endpoint and status
emma_csi_api_requests_total{method="POST",endpoint="/v1/volumes",status="200"} 42
emma_csi_api_requests_total{method="POST",endpoint="/v1/volumes",status="503"} 3

# API request latency
emma_csi_api_request_duration_seconds_sum{method="POST",endpoint="/v1/volumes"} 89.4
emma_csi_api_request_duration_seconds_count{method="POST",endpoint="/v1/volumes"} 45
```

**Interpretation**:
- API error rate: 3/45 = 6.7%
- Average API latency: 89.4/45 = 2.0 seconds

#### Volume State Metrics

```
# Volumes by status
emma_csi_volumes_total{status="AVAILABLE"} 15
emma_csi_volumes_total{status="ACTIVE"} 8
emma_csi_volumes_total{status="BUSY"} 2
```

**Interpretation**:
- 15 volumes provisioned but not attached
- 8 volumes currently attached to nodes
- 2 volumes in transitional state

#### Attachment Metrics

```
# Volume attachment latency
emma_csi_volume_attach_duration_seconds_sum 245.6
emma_csi_volume_attach_duration_seconds_count 18

# Volume detachment latency
emma_csi_volume_detach_duration_seconds_sum 89.2
emma_csi_volume_detach_duration_seconds_count 12
```

**Interpretation**:
- Average attach time: 245.6/18 = 13.6 seconds
- Average detach time: 89.2/12 = 7.4 seconds

### Alerting Thresholds

Recommended Prometheus alert rules:

```yaml
# High error rate
- alert: EmmaCSIHighErrorRate
  expr: rate(emma_csi_operations_total{status="failure"}[5m]) > 0.1
  annotations:
    summary: "Emma CSI driver error rate above 10%"

# Slow operations
- alert: EmmaCSISlowOperations
  expr: histogram_quantile(0.95, rate(emma_csi_operation_duration_seconds_bucket[5m])) > 30
  annotations:
    summary: "Emma CSI operations taking >30s at P95"

# API errors
- alert: EmmaAPIErrors
  expr: rate(emma_csi_api_requests_total{status=~"5.."}[5m]) > 0.05
  annotations:
    summary: "Emma API error rate above 5%"

# Controller down
- alert: EmmaCSIControllerDown
  expr: up{job="emma-csi-controller"} == 0
  annotations:
    summary: "Emma CSI controller is down"
```

## Advanced Debugging

### Inspecting CSI Communication

View CSI gRPC calls between Kubernetes and driver:

```bash
# Enable CSI sidecar debug logging
kubectl set env statefulset/emma-csi-controller -n kube-system -c csi-provisioner KLOG_V=5
kubectl set env daemonset/emma-csi-node -n kube-system -c csi-node-driver-registrar KLOG_V=5

# View sidecar logs
kubectl logs -n kube-system emma-csi-controller-0 -c csi-provisioner
kubectl logs -n kube-system <emma-csi-node-pod> -c csi-node-driver-registrar
```

### Debugging on Nodes

SSH to a node to inspect volume devices and mounts:

```bash
# List block devices
lsblk

# Check mounts
mount | grep emma

# Check CSI socket
ls -la /var/lib/kubelet/plugins/csi.emma.ms/

# Check staged volumes
ls -la /var/lib/kubelet/plugins/kubernetes.io/csi/pv/

# Check published volumes
ls -la /var/lib/kubelet/pods/*/volumes/kubernetes.io~csi/
```

### Manual API Testing

Test Emma API directly to isolate driver issues:

```bash
# Get access token
TOKEN=$(curl -X POST https://api.emma.ms/external/v1/issue-token \
  -H "Content-Type: application/json" \
  -d '{"clientId":"your-id","clientSecret":"your-secret"}' | jq -r '.accessToken')

# List volumes
curl -H "Authorization: Bearer $TOKEN" \
  https://api.emma.ms/external/v1/volumes

# Get volume details
curl -H "Authorization: Bearer $TOKEN" \
  https://api.emma.ms/external/v1/volumes/12345

# List VMs
curl -H "Authorization: Bearer $TOKEN" \
  https://api.emma.ms/external/v1/vms
```

### Collecting Debug Information

When reporting issues, collect the following:

```bash
# Create debug bundle
mkdir emma-csi-debug
cd emma-csi-debug

# Component status
kubectl get pods -n kube-system -l app.kubernetes.io/name=emma-csi-driver -o wide > pods.txt
kubectl get csidriver csi.emma.ms -o yaml > csidriver.yaml
kubectl get csistoragecapacities -o yaml > capacities.yaml

# Logs
kubectl logs -n kube-system emma-csi-controller-0 -c emma-csi-driver --tail=1000 > controller.log
kubectl logs -n kube-system emma-csi-controller-0 -c csi-provisioner --tail=1000 > provisioner.log

# Configuration
kubectl get configmap emma-csi-driver-config -n kube-system -o yaml > configmap.yaml
kubectl get storageclass -o yaml > storageclasses.yaml

# PVC/PV state
kubectl get pvc --all-namespaces -o yaml > pvcs.yaml
kubectl get pv -o yaml > pvs.yaml
kubectl get volumeattachment -o yaml > volumeattachments.yaml

# Events
kubectl get events --all-namespaces --sort-by='.lastTimestamp' > events.txt

# Create archive
cd ..
tar czf emma-csi-debug.tar.gz emma-csi-debug/
```

## Getting Help

If you cannot resolve the issue:

1. **Check documentation**:
   - [Installation Guide](INSTALLATION.md)
   - [User Guide](USER_GUIDE.md)
   - [API Reference](API_REFERENCE.md)

2. **Search existing issues**:
   - GitHub: https://github.com/your-org/emma-csi-driver/issues

3. **Create new issue**:
   - Include debug bundle
   - Describe expected vs actual behavior
   - Include relevant logs and metrics

4. **Contact support**:
   - Emma.ms Support: https://emma.ms/support
   - Driver maintainers: emma-csi-driver@your-org.com

