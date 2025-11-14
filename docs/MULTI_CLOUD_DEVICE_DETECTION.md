# Multi-Cloud Device Detection

## Overview

Emma.ms provisions VMs across multiple cloud providers (AWS, GCP, Azure), each with different block device naming conventions. The CSI driver now supports automatic detection of devices across all platforms.

## Cloud Provider Device Patterns

### AWS (Amazon Web Services)

**Device Type**: NVMe (Non-Volatile Memory Express)

**Symlink Patterns**:
```
/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol<hex_id>
/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol<hex_id>_1
```

**Actual Devices**:
```
/dev/nvme0n1  (root disk)
/dev/nvme1n1  (first attached volume)
/dev/nvme2n1  (second attached volume)
...
```

**Characteristics**:
- Fast NVMe interface
- Hex-based volume identifiers in symlinks
- No direct volume ID mapping in device name
- Must use modification time to identify newly attached volumes

**Example**:
```bash
$ ls -la /dev/disk/by-id/nvme-Amazon*
lrwxrwxrwx 1 root root 13 Nov 13 21:28 nvme-Amazon_Elastic_Block_Store_vol07ddf3bb0fd4a03e9 -> ../../nvme1n1
```

### GCP (Google Cloud Platform)

**Device Type**: SCSI (Small Computer System Interface)

**Symlink Patterns**:
```
/dev/disk/by-id/google-<disk-name>
/dev/disk/by-id/scsi-0Google_PersistentDisk_<disk-name>
```

**Actual Devices**:
```
/dev/sda  (root disk)
/dev/sdb  (first attached volume)
/dev/sdc  (second attached volume)
...
```

**Characteristics**:
- SCSI interface
- Disk name can be custom or auto-generated
- Google-specific identifiers in symlinks
- Predictable naming with disk names

**Example**:
```bash
$ ls -la /dev/disk/by-id/google-*
lrwxrwxrwx 1 root root 9 Nov 13 10:00 google-persistent-disk-1 -> ../../sdb
lrwxrwxrwx 1 root root 9 Nov 13 10:05 google-persistent-disk-2 -> ../../sdc
```

### Azure (Microsoft Azure)

**Device Type**: SCSI

**Symlink Patterns**:
```
/dev/disk/by-id/scsi-<wwn>
/dev/disk/by-lun/lun<number>
```

**Actual Devices**:
```
/dev/sda  (root disk)
/dev/sdb  (temporary disk)
/dev/sdc  (first attached volume)
/dev/sdd  (second attached volume)
...
```

**Characteristics**:
- SCSI interface
- LUN (Logical Unit Number) based identification
- WWN (World Wide Name) identifiers
- Temporary disk at /dev/sdb (ephemeral storage)

**Example**:
```bash
$ ls -la /dev/disk/by-id/scsi-*
lrwxrwxrwx 1 root root 9 Nov 13 10:00 scsi-36000c2900... -> ../../sdc
lrwxrwxrwx 1 root root 9 Nov 13 10:05 scsi-36000c2901... -> ../../sdd
```

### Legacy/KVM (Virtio)

**Device Type**: Virtio Block Device

**Symlink Patterns**:
```
/dev/disk/by-id/virtio-<volume-id>
```

**Actual Devices**:
```
/dev/vda  (root disk)
/dev/vdb  (first attached volume)
/dev/vdc  (second attached volume)
...
```

**Characteristics**:
- Virtio paravirtualized interface
- Direct volume ID in symlink
- Common in KVM/QEMU environments
- Simplest naming scheme

**Example**:
```bash
$ ls -la /dev/disk/by-id/virtio-*
lrwxrwxrwx 1 root root 9 Nov 13 10:00 virtio-93766 -> ../../vdb
lrwxrwxrwx 1 root root 9 Nov 13 10:05 virtio-93767 -> ../../vdc
```

## Detection Strategy

The CSI driver uses a multi-stage detection strategy:

### Stage 1: Direct Path Lookup (Fast)
Check known symlink patterns in order:
1. Virtio: `/dev/disk/by-id/virtio-<volumeID>`
2. GCP: `/dev/disk/by-id/google-<volumeID>`
3. GCP SCSI: `/dev/disk/by-id/scsi-0Google_PersistentDisk_<volumeID>`
4. Azure/Generic SCSI: `/dev/disk/by-id/scsi-<volumeID>`
5. Legacy QEMU: `/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_<volumeID>`

**Timeout**: 180 seconds with exponential backoff polling

### Stage 2: Serial Number Scan (Virtio)
For virtio devices, scan `/sys/block/vd*/serial` and `/sys/block/sd*/serial`:
- Read serial number from sysfs
- Match against volume ID
- Return device path if match found

### Stage 3: NVMe Device Scan (AWS)
For AWS NVMe devices:
- Glob `/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol*`
- Filter out partition links
- Select most recently modified device (newest attachment)
- Return resolved device path

### Stage 4: Cloud Provider Scan (GCP/Azure)
For GCP and Azure devices:
- Glob multiple patterns:
  - `/dev/disk/by-id/google-*`
  - `/dev/disk/by-id/scsi-0Google_PersistentDisk_*`
  - `/dev/disk/by-id/scsi-*`
- Filter out partition links
- Select most recently modified device
- Return resolved device path

### Stage 5: Timeout
If no device found after 180 seconds, return error.

## Implementation Details

### Key Functions

**`GetDevicePath(volumeID string)`**
- Main entry point for device discovery
- Implements Stage 1 (direct path lookup)
- Calls helper functions for other stages

**`findDeviceBySerial(volumeID string)`**
- Implements Stage 2 (serial number scan)
- Scans `/sys/block/` for matching serial numbers
- Works for virtio and some SCSI devices

**`findNVMeDevice(volumeID string)`**
- Implements Stage 3 (NVMe scan)
- AWS-specific detection
- Uses modification time heuristic

**`findCloudProviderDevice(volumeID string)`**
- Implements Stage 4 (GCP/Azure scan)
- Multi-pattern glob matching
- Uses modification time heuristic

### Modification Time Heuristic

For AWS, GCP, and Azure, we can't directly match volume IDs to device names. Instead, we use modification time:

**Rationale**:
- Newly attached devices have recent modification times
- Existing devices have older modification times
- The newest device is likely the one we just attached

**Limitations**:
- May fail if multiple volumes are attached simultaneously
- Relies on accurate system time
- Could select wrong device in rare race conditions

**Mitigation**:
- 180-second timeout allows for retry attempts
- Kubelet will retry mount operations
- Volume attachment is serialized by Kubernetes

## Testing Across Cloud Providers

### AWS Testing
```bash
# Check for NVMe devices
ls -la /dev/disk/by-id/nvme-Amazon*
lsblk -o NAME,SIZE,TYPE,MOUNTPOINT

# Verify device detection
kubectl logs -n kube-system -l app=emma-csi-node | grep "Found NVMe device"
```

### GCP Testing
```bash
# Check for Google devices
ls -la /dev/disk/by-id/google-*
ls -la /dev/disk/by-id/scsi-0Google*
lsblk -o NAME,SIZE,TYPE,MOUNTPOINT

# Verify device detection
kubectl logs -n kube-system -l app=emma-csi-node | grep "Found cloud provider device"
```

### Azure Testing
```bash
# Check for SCSI devices
ls -la /dev/disk/by-id/scsi-*
ls -la /dev/disk/by-lun/
lsblk -o NAME,SIZE,TYPE,MOUNTPOINT

# Verify device detection
kubectl logs -n kube-system -l app=emma-csi-node | grep "Found cloud provider device"
```

### Legacy/KVM Testing
```bash
# Check for virtio devices
ls -la /dev/disk/by-id/virtio-*
lsblk -o NAME,SIZE,TYPE,MOUNTPOINT

# Verify device detection
kubectl logs -n kube-system -l app=emma-csi-node | grep "Found device.*by serial"
```

## Troubleshooting

### Device Not Found

**Symptoms**:
```
timeout waiting for device for volume X
```

**Diagnosis**:
1. Check if device is attached at OS level:
   ```bash
   lsblk
   ls -la /dev/disk/by-id/
   ```

2. Check CSI node logs:
   ```bash
   kubectl logs -n kube-system -l app=emma-csi-node --tail=100
   ```

3. Verify volume is attached via Emma API:
   ```bash
   # Check volume status in Emma dashboard
   ```

**Solutions**:
- Wait for device to appear (may take 30-60s)
- Trigger udev rescan: `udevadm trigger && udevadm settle`
- Check for VM state conflicts in controller logs
- Verify volume is attached to correct VM

### Wrong Device Selected

**Symptoms**:
- Mount succeeds but wrong data appears
- Multiple volumes attached simultaneously

**Diagnosis**:
1. Check device modification times:
   ```bash
   ls -la --time-style=full-iso /dev/disk/by-id/
   ```

2. Check volume attachment order:
   ```bash
   kubectl get volumeattachments
   ```

**Solutions**:
- Avoid attaching multiple volumes simultaneously
- Use explicit device paths if available
- Check Emma API for correct volume-to-VM mapping

## Future Improvements

1. **Volume ID Mapping**: Store volume ID in device metadata for direct lookup
2. **Udev Events**: Use udev events instead of polling for faster detection
3. **Cloud Provider Detection**: Auto-detect cloud provider from VM metadata
4. **Parallel Attachment**: Better handling of simultaneous volume attachments
5. **Device Verification**: Verify device size matches expected volume size
