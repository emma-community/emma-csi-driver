// +build e2e

package e2e

import (
	"context"
	"os"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// getKubernetesClient creates a Kubernetes client from kubeconfig
func getKubernetesClient(t *testing.T) *kubernetes.Clientset {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		kubeconfig = os.Getenv("HOME") + "/.kube/config"
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("failed to build kubeconfig: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatalf("failed to create kubernetes client: %v", err)
	}

	return clientset
}

// TestPVCProvisioning tests PVC provisioning end-to-end
// This test requires:
// - A running Kubernetes cluster
// - Emma CSI driver deployed
// - Emma API credentials configured
// Run with: go test -tags=e2e ./test/e2e/...
func TestPVCProvisioning(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping e2e test: E2E_TEST not set")
	}

	client := getKubernetesClient(t)
	ctx := context.Background()
	namespace := "default"
	testName := "emma-csi-test-" + time.Now().Format("20060102-150405")

	// Create PVC
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr("emma-ssd"),
		},
	}

	t.Logf("Creating PVC: %s", testName)
	_, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create PVC: %v", err)
	}

	// Clean up
	defer func() {
		t.Logf("Cleaning up PVC: %s", testName)
		_ = client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, testName, metav1.DeleteOptions{})
	}()

	// Wait for PVC to be bound
	t.Log("Waiting for PVC to be bound...")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for PVC to be bound")
		case <-ticker.C:
			pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, testName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get PVC: %v", err)
			}
			if pvc.Status.Phase == v1.ClaimBound {
				t.Logf("PVC bound to PV: %s", pvc.Spec.VolumeName)
				return
			}
			t.Logf("PVC status: %s", pvc.Status.Phase)
		}
	}
}

// TestPodMounting tests pod mounting with Emma CSI volume
func TestPodMounting(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping e2e test: E2E_TEST not set")
	}

	client := getKubernetesClient(t)
	ctx := context.Background()
	namespace := "default"
	testName := "emma-csi-pod-test-" + time.Now().Format("20060102-150405")

	// Create PVC
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr("emma-ssd"),
		},
	}

	t.Logf("Creating PVC: %s", testName)
	_, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create PVC: %v", err)
	}

	// Clean up
	defer func() {
		t.Logf("Cleaning up resources")
		_ = client.CoreV1().Pods(namespace).Delete(ctx, testName, metav1.DeleteOptions{})
		_ = client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, testName, metav1.DeleteOptions{})
	}()

	// Create pod using the PVC
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: namespace,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:  "test-container",
					Image: "busybox",
					Command: []string{
						"sh",
						"-c",
						"echo 'Hello from Emma CSI' > /data/test.txt && sleep 3600",
					},
					VolumeMounts: []v1.VolumeMount{
						{
							Name:      "test-volume",
							MountPath: "/data",
						},
					},
				},
			},
			Volumes: []v1.Volume{
				{
					Name: "test-volume",
					VolumeSource: v1.VolumeSource{
						PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
							ClaimName: testName,
						},
					},
				},
			},
		},
	}

	t.Logf("Creating pod: %s", testName)
	_, err = client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create pod: %v", err)
	}

	// Wait for pod to be running
	t.Log("Waiting for pod to be running...")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for pod to be running")
		case <-ticker.C:
			pod, err := client.CoreV1().Pods(namespace).Get(ctx, testName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get pod: %v", err)
			}
			if pod.Status.Phase == v1.PodRunning {
				t.Log("Pod is running successfully")
				return
			}
			t.Logf("Pod status: %s", pod.Status.Phase)
		}
	}
}

// TestVolumeExpansion tests volume expansion end-to-end
func TestVolumeExpansion(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping e2e test: E2E_TEST not set")
	}

	client := getKubernetesClient(t)
	ctx := context.Background()
	namespace := "default"
	testName := "emma-csi-expand-test-" + time.Now().Format("20060102-150405")

	// Create PVC
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr("emma-ssd"),
		},
	}

	t.Logf("Creating PVC: %s", testName)
	createdPVC, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create PVC: %v", err)
	}

	// Clean up
	defer func() {
		t.Logf("Cleaning up PVC: %s", testName)
		_ = client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, testName, metav1.DeleteOptions{})
	}()

	// Wait for PVC to be bound
	t.Log("Waiting for PVC to be bound...")
	time.Sleep(30 * time.Second)

	// Expand PVC
	t.Log("Expanding PVC to 20Gi...")
	createdPVC.Spec.Resources.Requests[v1.ResourceStorage] = resource.MustParse("20Gi")
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, createdPVC, metav1.UpdateOptions{})
	if err != nil {
		t.Fatalf("failed to expand PVC: %v", err)
	}

	// Wait for expansion to complete
	t.Log("Waiting for expansion to complete...")
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for volume expansion")
		case <-ticker.C:
			pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, testName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("failed to get PVC: %v", err)
			}
			capacity := pvc.Status.Capacity[v1.ResourceStorage]
			t.Logf("Current capacity: %s", capacity.String())
			if capacity.Cmp(resource.MustParse("20Gi")) >= 0 {
				t.Log("Volume expansion completed successfully")
				return
			}
		}
	}
}

// TestCleanup tests proper cleanup of resources
func TestCleanup(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Skipping e2e test: E2E_TEST not set")
	}

	client := getKubernetesClient(t)
	ctx := context.Background()
	namespace := "default"
	testName := "emma-csi-cleanup-test-" + time.Now().Format("20060102-150405")

	// Create PVC
	pvc := &v1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      testName,
			Namespace: namespace,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{
					v1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
			StorageClassName: stringPtr("emma-ssd"),
		},
	}

	t.Logf("Creating PVC: %s", testName)
	_, err := client.CoreV1().PersistentVolumeClaims(namespace).Create(ctx, pvc, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("failed to create PVC: %v", err)
	}

	// Wait for PVC to be bound
	time.Sleep(30 * time.Second)

	// Delete PVC
	t.Logf("Deleting PVC: %s", testName)
	err = client.CoreV1().PersistentVolumeClaims(namespace).Delete(ctx, testName, metav1.DeleteOptions{})
	if err != nil {
		t.Fatalf("failed to delete PVC: %v", err)
	}

	// Wait for PVC to be deleted
	t.Log("Waiting for PVC to be deleted...")
	timeout := time.After(2 * time.Minute)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			t.Fatal("timeout waiting for PVC deletion")
		case <-ticker.C:
			_, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, testName, metav1.GetOptions{})
			if err != nil {
				// PVC not found - successfully deleted
				t.Log("PVC deleted successfully")
				return
			}
			t.Log("Waiting for PVC deletion...")
		}
	}
}

func stringPtr(s string) *string {
	return &s
}
