package probe

import (
	"context"
	"testing"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func int32Ptr(i int32) *int32 { return &i }

func TestK8sProbeDeploymentReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 3,
		},
	})

	p := NewK8sProbe(K8sProbeConfig{
		Name:         "deploy-ready",
		Mode:         domain.ProbeModeSOT,
		Clientset:    cs,
		Namespace:    "default",
		ResourceKind: "deployment",
		ResourceName: "web",
	})

	assert.Equal(t, "deploy-ready", p.Name())
	assert.Equal(t, "k8s", p.Type())
	assert.Equal(t, domain.ProbeModeSOT, p.Mode())

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Passed)
	assert.Equal(t, int32(3), result.Detail["desired_replicas"])
	assert.Equal(t, int32(3), result.Detail["ready_replicas"])
}

func TestK8sProbeDeploymentNotReady(t *testing.T) {
	cs := fake.NewSimpleClientset(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	})

	p := NewK8sProbe(K8sProbeConfig{
		Name:         "deploy-not-ready",
		Mode:         domain.ProbeModeSOT,
		Clientset:    cs,
		ResourceKind: "deployment",
		ResourceName: "web",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestK8sProbeDeploymentNotFound(t *testing.T) {
	cs := fake.NewSimpleClientset()

	p := NewK8sProbe(K8sProbeConfig{
		Name:         "missing",
		Mode:         domain.ProbeModeSOT,
		Clientset:    cs,
		ResourceKind: "deployment",
		ResourceName: "nonexistent",
	})

	_, err := p.Execute(context.Background())
	assert.Error(t, err)
}

func TestK8sProbePodRunning(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "worker-1",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	})

	p := NewK8sProbe(K8sProbeConfig{
		Name:         "pod-running",
		Mode:         domain.ProbeModeSOT,
		Clientset:    cs,
		ResourceKind: "pod",
		ResourceName: "worker-1",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)

	assert.True(t, result.Passed)
	assert.Equal(t, "Running", result.Detail["phase"])
	assert.Equal(t, "Running", result.Detail["expected_phase"])
}

func TestK8sProbePodCustomExpected(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "batch-job",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	})

	p := NewK8sProbe(K8sProbeConfig{
		Name:          "pod-succeeded",
		Mode:          domain.ProbeModeSOT,
		Clientset:     cs,
		ResourceKind:  "pod",
		ResourceName:  "batch-job",
		ExpectedValue: "Succeeded",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.True(t, result.Passed)
}

func TestK8sProbePodWrongPhase(t *testing.T) {
	cs := fake.NewSimpleClientset(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failing",
			Namespace: "default",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	})

	p := NewK8sProbe(K8sProbeConfig{
		Name:         "pod-failed",
		Mode:         domain.ProbeModeSOT,
		Clientset:    cs,
		ResourceKind: "pod",
		ResourceName: "failing",
	})

	result, err := p.Execute(context.Background())
	require.NoError(t, err)
	assert.False(t, result.Passed)
}

func TestK8sProbeUnsupportedKind(t *testing.T) {
	cs := fake.NewSimpleClientset()

	p := NewK8sProbe(K8sProbeConfig{
		Name:         "bad-kind",
		Mode:         domain.ProbeModeSOT,
		Clientset:    cs,
		ResourceKind: "statefulset",
		ResourceName: "test",
	})

	_, err := p.Execute(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported resource kind")
}

func TestK8sProbeDefaultNamespace(t *testing.T) {
	p := NewK8sProbe(K8sProbeConfig{
		Name:         "default-ns",
		Mode:         domain.ProbeModeSOT,
		Clientset:    fake.NewSimpleClientset(),
		ResourceKind: "deployment",
		ResourceName: "test",
	})

	assert.Equal(t, "default", p.namespace)
}
