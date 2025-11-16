package mount

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"k8s.io/klog/v2"
)

// Mounter provides mount operations
type Mounter interface {
	// Mount mounts source to target with the given fstype and options
	Mount(source, target, fstype string, options []string) error

	// Unmount unmounts the target
	Unmount(target string) error

	// IsLikelyNotMountPoint checks if a path is not a mount point
	IsLikelyNotMountPoint(path string) (bool, error)

	// FormatAndMount formats the device and mounts it
	FormatAndMount(source, target, fstype string, options []string) error

	// GetDevicePath discovers the device path for a volume
	GetDevicePath(volumeID string) (string, error)

	// ResizeFilesystem resizes the filesystem on the device
	ResizeFilesystem(devicePath, fstype string) error

	// GetVolumeStats returns volume statistics
	GetVolumeStats(path string) (*VolumeStats, error)
}

// VolumeStats represents volume usage statistics
type VolumeStats struct {
	AvailableBytes  int64
	TotalBytes      int64
	UsedBytes       int64
	AvailableInodes int64
	TotalInodes     int64
	UsedInodes      int64
}

// LinuxMounter implements Mounter for Linux systems
type LinuxMounter struct{}

// NewMounter creates a new mounter
func NewMounter() Mounter {
	return &LinuxMounter{}
}

// Mount mounts source to target
func (m *LinuxMounter) Mount(source, target, fstype string, options []string) error {
	klog.V(4).Infof("Mounting %s to %s with fstype %s and options %v", source, target, fstype, options)

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(target, 0750); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Build mount command
	args := []string{}
	if fstype != "" {
		args = append(args, "-t", fstype)
	}
	if len(options) > 0 {
		args = append(args, "-o", strings.Join(options, ","))
	}
	args = append(args, source, target)

	cmd := exec.Command("mount", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mount failed: %w, output: %s", err, string(output))
	}

	klog.V(4).Infof("Successfully mounted %s to %s", source, target)
	return nil
}

// Unmount unmounts the target
func (m *LinuxMounter) Unmount(target string) error {
	klog.V(4).Infof("Unmounting %s", target)

	cmd := exec.Command("umount", target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("unmount failed: %w, output: %s", err, string(output))
	}

	klog.V(4).Infof("Successfully unmounted %s", target)
	return nil
}

// IsLikelyNotMountPoint checks if a path is not a mount point
func (m *LinuxMounter) IsLikelyNotMountPoint(path string) (bool, error) {
	// Check if path exists
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, err
	}

	// Use findmnt to check if it's a mount point
	cmd := exec.Command("findmnt", "-o", "TARGET", "-n", "-M", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If findmnt returns error, it's likely not a mount point
		return true, nil
	}

	// If output is not empty, it's a mount point
	return len(strings.TrimSpace(string(output))) == 0, nil
}

// FormatAndMount formats the device and mounts it
func (m *LinuxMounter) FormatAndMount(source, target, fstype string, options []string) error {
	klog.V(4).Infof("Formatting and mounting %s to %s with fstype %s", source, target, fstype)

	// Check if device is already formatted
	existingFS, err := m.getFilesystemType(source)
	if err != nil {
		return fmt.Errorf("failed to check existing filesystem: %w", err)
	}

	// Format if not already formatted or if filesystem type doesn't match
	if existingFS == "" || existingFS != fstype {
		klog.V(4).Infof("Formatting device %s with %s", source, fstype)
		if err := m.formatDevice(source, fstype); err != nil {
			return fmt.Errorf("failed to format device: %w", err)
		}
	} else {
		klog.V(4).Infof("Device %s already formatted with %s", source, fstype)
	}

	// Mount the device
	return m.Mount(source, target, fstype, options)
}

// getFilesystemType returns the filesystem type of a device
func (m *LinuxMounter) getFilesystemType(device string) (string, error) {
	cmd := exec.Command("blkid", "-o", "value", "-s", "TYPE", device)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Device might not be formatted yet
		return "", nil
	}

	return strings.TrimSpace(string(output)), nil
}

// formatDevice formats a device with the specified filesystem
func (m *LinuxMounter) formatDevice(device, fstype string) error {
	var cmd *exec.Cmd

	switch fstype {
	case "ext4":
		// -F forces formatting without prompting
		cmd = exec.Command("mkfs.ext4", "-F", device)
	case "xfs":
		// -f forces formatting
		cmd = exec.Command("mkfs.xfs", "-f", device)
	default:
		return fmt.Errorf("unsupported filesystem type: %s", fstype)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("format failed: %w, output: %s", err, string(output))
	}

	return nil
}

// GetDevicePath discovers the device path for a volume
func (m *LinuxMounter) GetDevicePath(volumeID string) (string, error) {
	klog.V(4).Infof("Discovering device path for volume %s", volumeID)

	// Emma.ms provisions VMs on different cloud providers, each with different device naming:
	//
	// AWS: NVMe devices
	//   - /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol*
	//   - Actual device: /dev/nvme1n1, /dev/nvme2n1, etc.
	//
	// GCP: SCSI devices with Google naming
	//   - /dev/disk/by-id/google-<disk-name>
	//   - /dev/disk/by-id/scsi-0Google_PersistentDisk_*
	//   - Actual device: /dev/sdb, /dev/sdc, etc.
	//
	// Azure: SCSI devices with Microsoft naming
	//   - /dev/disk/by-id/scsi-*
	//   - /dev/disk/by-lun/* (LUN-based)
	//   - Actual device: /dev/sdc, /dev/sdd, etc.
	//
	// Legacy/KVM: Virtio devices
	//   - /dev/disk/by-id/virtio-<volumeID>
	//   - Actual device: /dev/vdb, /dev/vdc, etc.

	// Primary device path for virtio (legacy/KVM)
	primaryPath := "/dev/disk/by-id/virtio-" + volumeID

	// Alternative paths to check for different cloud providers
	alternativePaths := []string{
		// GCP patterns
		"/dev/disk/by-id/google-" + volumeID,
		"/dev/disk/by-id/scsi-0Google_PersistentDisk_" + volumeID,

		// Azure patterns (SCSI)
		"/dev/disk/by-id/scsi-" + volumeID,

		// Legacy QEMU/KVM patterns
		"/dev/disk/by-id/scsi-0QEMU_QEMU_HARDDISK_" + volumeID,
		"/dev/disk/by-id/ata-QEMU_HARDDISK_" + volumeID,
	}

	// Wait for device to appear (up to 180 seconds with exponential backoff)
	// This needs to be longer than the controller's attach retry timeout
	// to account for VM state conflicts and API delays
	maxWait := 180 * time.Second
	deadline := time.Now().Add(maxWait)
	checkInterval := 500 * time.Millisecond

	klog.V(4).Infof("Waiting for device to appear at %s (timeout: %v)", primaryPath, maxWait)

	for time.Now().Before(deadline) {
		// First, try the primary path
		if _, err := os.Stat(primaryPath); err == nil {
			if m.isBlockDevice(primaryPath) {
				// Resolve symlink to get actual device
				realPath, err := filepath.EvalSymlinks(primaryPath)
				if err == nil {
					klog.V(4).Infof("Found device %s -> %s for volume %s", primaryPath, realPath, volumeID)
					return realPath, nil
				}
				klog.V(4).Infof("Found device %s for volume %s", primaryPath, volumeID)
				return primaryPath, nil
			}
		}

		// Try alternative paths
		for _, altPath := range alternativePaths {
			if _, err := os.Stat(altPath); err == nil {
				if m.isBlockDevice(altPath) {
					realPath, err := filepath.EvalSymlinks(altPath)
					if err == nil {
						klog.V(4).Infof("Found device %s -> %s for volume %s", altPath, realPath, volumeID)
						return realPath, nil
					}
					klog.V(4).Infof("Found device %s for volume %s", altPath, volumeID)
					return altPath, nil
				}
			}
		}

		// Trigger udev to rescan devices
		if time.Now().Add(checkInterval).After(deadline) {
			// Last attempt - trigger udev rescan
			klog.V(5).Info("Triggering udev rescan")
			_ = exec.Command("udevadm", "trigger").Run()
			_ = exec.Command("udevadm", "settle").Run()
		}

		time.Sleep(checkInterval)

		// Increase check interval gradually (exponential backoff)
		if checkInterval < 5*time.Second {
			checkInterval = checkInterval * 2
		}
	}

	// Last resort: scan all block devices and try to match by serial or cloud provider patterns
	klog.V(4).Infof("Device not found at expected paths, scanning all block devices")

	// Try to find by serial (virtio devices)
	if device, err := m.findDeviceBySerial(volumeID); err == nil {
		klog.V(4).Infof("Found device %s by serial scan for volume %s", device, volumeID)
		return device, nil
	}

	// Try to find NVMe device (AWS)
	if device, err := m.findNVMeDevice(volumeID); err == nil {
		klog.V(4).Infof("Found NVMe device %s for volume %s", device, volumeID)
		return device, nil
	}

	// Try to find cloud provider device (GCP, Azure, etc.)
	if device, err := m.findCloudProviderDevice(volumeID); err == nil {
		klog.V(4).Infof("Found cloud provider device %s for volume %s", device, volumeID)
		return device, nil
	}

	return "", fmt.Errorf("timeout waiting for device for volume %s", volumeID)
}

// findDeviceBySerial scans all block devices to find one matching the volume ID
func (m *LinuxMounter) findDeviceBySerial(volumeID string) (string, error) {
	// Check /sys/block for all block devices
	blockDevices, err := filepath.Glob("/sys/block/vd*")
	if err == nil {
		for _, sysPath := range blockDevices {
			serialPath := filepath.Join(sysPath, "serial")
			if data, err := os.ReadFile(serialPath); err == nil {
				serial := strings.TrimSpace(string(data))
				if serial == volumeID {
					deviceName := filepath.Base(sysPath)
					devicePath := "/dev/" + deviceName
					if m.isBlockDevice(devicePath) {
						return devicePath, nil
					}
				}
			}
		}
	}

	// Also check sd* devices
	blockDevices, err = filepath.Glob("/sys/block/sd*")
	if err == nil {
		for _, sysPath := range blockDevices {
			serialPath := filepath.Join(sysPath, "serial")
			if data, err := os.ReadFile(serialPath); err == nil {
				serial := strings.TrimSpace(string(data))
				if serial == volumeID {
					deviceName := filepath.Base(sysPath)
					devicePath := "/dev/" + deviceName
					if m.isBlockDevice(devicePath) {
						return devicePath, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("device not found for volume %s", volumeID)
}

// findNVMeDevice scans /dev/disk/by-id/ for NVMe devices that might match the volume
func (m *LinuxMounter) findNVMeDevice(volumeID string) (string, error) {
	// On Emma.ms with AWS-style NVMe, devices appear as:
	// /dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol<hex_id>
	// We need to scan all NVMe devices and check which one was attached most recently
	// or matches our volume ID in some way

	pattern := "/dev/disk/by-id/nvme-Amazon_Elastic_Block_Store_vol*"
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to glob NVMe devices: %w", err)
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no NVMe devices found")
	}

	// Find the most recently created device (likely our newly attached volume)
	var newestDevice string
	var newestTime time.Time

	for _, match := range matches {
		// Skip partition links (they contain -part)
		if strings.Contains(match, "-part") || strings.Contains(match, "_1") {
			continue
		}

		info, err := os.Lstat(match)
		if err != nil {
			continue
		}

		modTime := info.ModTime()
		if newestDevice == "" || modTime.After(newestTime) {
			// Resolve symlink to get actual device
			if realPath, err := filepath.EvalSymlinks(match); err == nil {
				if m.isBlockDevice(realPath) {
					newestDevice = realPath
					newestTime = modTime
					klog.V(5).Infof("Found NVMe candidate: %s -> %s (mtime: %v)", match, realPath, modTime)
				}
			}
		}
	}

	if newestDevice != "" {
		klog.V(4).Infof("Selected newest NVMe device: %s (mtime: %v)", newestDevice, newestTime)
		return newestDevice, nil
	}

	return "", fmt.Errorf("no suitable NVMe device found")
}

// findCloudProviderDevice scans for devices from GCP, Azure, or other cloud providers
func (m *LinuxMounter) findCloudProviderDevice(volumeID string) (string, error) {
	klog.V(4).Infof("Scanning for cloud provider devices for volume %s", volumeID)

	// Patterns to check for different cloud providers
	patterns := []string{
		// GCP patterns
		"/dev/disk/by-id/google-*",
		"/dev/disk/by-id/scsi-0Google_PersistentDisk_*",

		// Azure patterns
		"/dev/disk/by-id/scsi-*",

		// Generic SCSI patterns
		"/dev/disk/by-id/scsi-3*",
	}

	var allMatches []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err == nil {
			allMatches = append(allMatches, matches...)
		}
	}

	if len(allMatches) == 0 {
		return "", fmt.Errorf("no cloud provider devices found")
	}

	// Find the most recently created device (likely our newly attached volume)
	var newestDevice string
	var newestTime time.Time

	for _, match := range allMatches {
		// Skip partition links
		if strings.Contains(match, "-part") || strings.Contains(match, "_1") {
			continue
		}

		info, err := os.Lstat(match)
		if err != nil {
			continue
		}

		modTime := info.ModTime()
		if newestDevice == "" || modTime.After(newestTime) {
			// Resolve symlink to get actual device
			if realPath, err := filepath.EvalSymlinks(match); err == nil {
				if m.isBlockDevice(realPath) {
					newestDevice = realPath
					newestTime = modTime
					klog.V(5).Infof("Found cloud device candidate: %s -> %s (mtime: %v)", match, realPath, modTime)
				}
			}
		}
	}

	if newestDevice != "" {
		klog.V(4).Infof("Selected newest cloud provider device: %s (mtime: %v)", newestDevice, newestTime)
		return newestDevice, nil
	}

	return "", fmt.Errorf("no suitable cloud provider device found")
}

// isBlockDevice checks if a path is a block device
func (m *LinuxMounter) isBlockDevice(path string) bool {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false
	}

	// Check if it's a block device
	mode := fileInfo.Mode()
	return mode&os.ModeDevice != 0 && mode&os.ModeCharDevice == 0
}

// ResizeFilesystem resizes the filesystem on the device
func (m *LinuxMounter) ResizeFilesystem(devicePath, fstype string) error {
	klog.V(4).Infof("Resizing filesystem on %s (type: %s)", devicePath, fstype)

	var cmd *exec.Cmd

	switch fstype {
	case "ext4":
		// resize2fs for ext4
		cmd = exec.Command("resize2fs", devicePath)
	case "xfs":
		// xfs_growfs for xfs (requires mount point, not device)
		// This should be called with the mount point, not the device
		return fmt.Errorf("xfs resize requires mount point, not device path")
	default:
		return fmt.Errorf("unsupported filesystem type for resize: %s", fstype)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resize failed: %w, output: %s", err, string(output))
	}

	klog.V(4).Infof("Successfully resized filesystem on %s", devicePath)
	return nil
}

// ResizeFilesystemAtPath resizes the filesystem at the given mount path
func (m *LinuxMounter) ResizeFilesystemAtPath(mountPath, fstype string) error {
	klog.V(4).Infof("Resizing filesystem at %s (type: %s)", mountPath, fstype)

	var cmd *exec.Cmd

	switch fstype {
	case "xfs":
		// xfs_growfs requires the mount point
		cmd = exec.Command("xfs_growfs", mountPath)
	default:
		return fmt.Errorf("ResizeFilesystemAtPath only supports xfs, got: %s", fstype)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("resize failed: %w, output: %s", err, string(output))
	}

	klog.V(4).Infof("Successfully resized filesystem at %s", mountPath)
	return nil
}

// GetVolumeStats returns volume statistics
func (m *LinuxMounter) GetVolumeStats(path string) (*VolumeStats, error) {
	// Use df to get volume statistics
	cmd := exec.Command("df", "--block-size=1", "--output=size,used,avail", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get volume stats: %w, output: %s", err, string(output))
	}

	// Parse output (skip header line)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected df output: %s", string(output))
	}

	fields := strings.Fields(lines[1])
	if len(fields) < 3 {
		return nil, fmt.Errorf("unexpected df output format: %s", lines[1])
	}

	var total, used, available int64
	fmt.Sscanf(fields[0], "%d", &total)
	fmt.Sscanf(fields[1], "%d", &used)
	fmt.Sscanf(fields[2], "%d", &available)

	// Get inode statistics
	cmd = exec.Command("df", "--output=itotal,iused,iavail", path)
	output, err = cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get inode stats: %w, output: %s", err, string(output))
	}

	lines = strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("unexpected df inode output: %s", string(output))
	}

	fields = strings.Fields(lines[1])
	if len(fields) < 3 {
		return nil, fmt.Errorf("unexpected df inode output format: %s", lines[1])
	}

	var totalInodes, usedInodes, availableInodes int64
	fmt.Sscanf(fields[0], "%d", &totalInodes)
	fmt.Sscanf(fields[1], "%d", &usedInodes)
	fmt.Sscanf(fields[2], "%d", &availableInodes)

	return &VolumeStats{
		TotalBytes:      total,
		UsedBytes:       used,
		AvailableBytes:  available,
		TotalInodes:     totalInodes,
		UsedInodes:      usedInodes,
		AvailableInodes: availableInodes,
	}, nil
}
