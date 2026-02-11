package probe

import (
	"context"
	"fmt"
	"time"

	"github.com/chaosduck/backend-go/internal/domain"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// K8sProbe checks Kubernetes resource state (deployment readiness, pod phase)
type K8sProbe struct {
	name          string
	mode          domain.ProbeMode
	clientset     kubernetes.Interface
	namespace     string
	resourceKind  string
	resourceName  string
	condition     string
	expectedValue string
}

// K8sProbeConfig holds construction parameters for K8sProbe
type K8sProbeConfig struct {
	Name          string
	Mode          domain.ProbeMode
	Clientset     kubernetes.Interface
	Namespace     string
	ResourceKind  string
	ResourceName  string
	Condition     string
	ExpectedValue string
}

// NewK8sProbe creates a Kubernetes resource probe
func NewK8sProbe(cfg K8sProbeConfig) *K8sProbe {
	if cfg.Namespace == "" {
		cfg.Namespace = "default"
	}
	if cfg.Condition == "" {
		cfg.Condition = "ready"
	}
	return &K8sProbe{
		name:          cfg.Name,
		mode:          cfg.Mode,
		clientset:     cfg.Clientset,
		namespace:     cfg.Namespace,
		resourceKind:  cfg.ResourceKind,
		resourceName:  cfg.ResourceName,
		condition:     cfg.Condition,
		expectedValue: cfg.ExpectedValue,
	}
}

func (p *K8sProbe) Name() string          { return p.name }
func (p *K8sProbe) Type() string          { return "k8s" }
func (p *K8sProbe) Mode() domain.ProbeMode { return p.mode }

func (p *K8sProbe) Execute(ctx context.Context) (*ProbeResult, error) {
	switch p.resourceKind {
	case "deployment":
		return p.checkDeployment(ctx)
	case "pod":
		return p.checkPod(ctx)
	default:
		return nil, fmt.Errorf("unsupported resource kind: %s", p.resourceKind)
	}
}

func (p *K8sProbe) checkDeployment(ctx context.Context) (*ProbeResult, error) {
	dep, err := p.clientset.AppsV1().Deployments(p.namespace).Get(ctx, p.resourceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get deployment: %w", err)
	}

	desired := int32(0)
	if dep.Spec.Replicas != nil {
		desired = *dep.Spec.Replicas
	}
	ready := dep.Status.ReadyReplicas

	passed := ready == desired

	return &ProbeResult{
		ProbeName: p.name,
		ProbeType: "k8s",
		Mode:      p.mode,
		Passed:    passed,
		Detail: map[string]any{
			"deployment":       p.resourceName,
			"namespace":        p.namespace,
			"desired_replicas": desired,
			"ready_replicas":   ready,
			"condition":        p.condition,
		},
		ExecutedAt: time.Now().UTC(),
	}, nil
}

func (p *K8sProbe) checkPod(ctx context.Context) (*ProbeResult, error) {
	pod, err := p.clientset.CoreV1().Pods(p.namespace).Get(ctx, p.resourceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod: %w", err)
	}

	phase := string(pod.Status.Phase)
	expected := p.expectedValue
	if expected == "" {
		expected = "Running"
	}
	passed := phase == expected

	return &ProbeResult{
		ProbeName: p.name,
		ProbeType: "k8s",
		Mode:      p.mode,
		Passed:    passed,
		Detail: map[string]any{
			"pod":            p.resourceName,
			"namespace":      p.namespace,
			"phase":          phase,
			"expected_phase": expected,
		},
		ExecutedAt: time.Now().UTC(),
	}, nil
}
