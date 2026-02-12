package engine

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/safety"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
)

// K8sEngine implements chaos operations against a Kubernetes cluster.
// All mutation methods return (result, rollbackFn).
type K8sEngine struct {
	clientset   kubernetes.Interface
	restConfig  *rest.Config
	esm         *safety.EmergencyStopManager
}

// NewK8sEngine creates a K8sEngine with in-cluster or kubeconfig auth
func NewK8sEngine(kubeconfig string, esm *safety.EmergencyStopManager) (*K8sEngine, error) {
	var cfg *rest.Config
	var err error

	if kubeconfig != "" {
		cfg, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		cfg, err = rest.InClusterConfig()
		if err != nil {
			// Fallback to default kubeconfig
			cfg, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		}
	}
	if err != nil {
		return nil, fmt.Errorf("k8s config: %w", err)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("k8s clientset: %w", err)
	}

	return &K8sEngine{clientset: cs, restConfig: cfg, esm: esm}, nil
}

// Clientset exposes the underlying kubernetes.Interface for probes
func (e *K8sEngine) Clientset() kubernetes.Interface {
	return e.clientset
}

func (e *K8sEngine) checkEmergencyStop() error {
	return e.esm.CheckEmergencyStop()
}

// PodDelete deletes pods matching the label selector
func (e *K8sEngine) PodDelete(ctx context.Context, namespace, labelSelector string, cfg *domain.ExperimentConfig) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	podNames := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		podNames = append(podNames, p.Name)
	}

	// Blast radius check
	allPods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list all pods: %w", err)
	}
	maxRatio := 0.3
	if cfg != nil {
		maxRatio = cfg.Safety.MaxBlastRadius
	}
	if err := safety.ValidateBlastRadius(len(podNames), len(allPods.Items), maxRatio); err != nil {
		return nil, fmt.Errorf("%w: %d/%d pods", err, len(podNames), len(allPods.Items))
	}

	if cfg != nil && cfg.Safety.DryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "pod_delete", "pods": podNames, "dry_run": true},
		}, nil
	}

	// Delete pods and save specs for rollback
	deletedPods := make([]corev1.Pod, 0, len(pods.Items))
	for _, pod := range pods.Items {
		if err := e.clientset.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{}); err != nil {
			// Partial failure: return rollback for already-deleted pods
			log.Printf("Failed to delete pod %s (deleted %d/%d): %v", pod.Name, len(deletedPods), len(pods.Items), err)
			rollback := buildPodRollback(e.clientset, namespace, deletedPods)
			return &domain.ChaosResult{
				Result:     map[string]any{"action": "pod_delete", "pods": podNameListFromPods(deletedPods), "partial_failure": pod.Name},
				RollbackFn: rollback,
			}, fmt.Errorf("delete pod %s: %w", pod.Name, err)
		}
		deletedPods = append(deletedPods, pod)
	}
	log.Printf("Deleted %d pods in %s", len(deletedPods), namespace)

	rollback := buildPodRollback(e.clientset, namespace, deletedPods)

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "pod_delete", "pods": podNames},
		RollbackFn: rollback,
	}, nil
}

// NetworkLatency injects network latency using tc in pod containers
func (e *K8sEngine) NetworkLatency(ctx context.Context, namespace, labelSelector string, latencyMs int, cfg *domain.ExperimentConfig) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	podNames := podNameList(pods)

	if cfg != nil && cfg.Safety.DryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "network_latency", "pods": podNames, "latency_ms": latencyMs, "dry_run": true},
		}, nil
	}

	for _, pod := range pods.Items {
		if _, err := e.execInPod(ctx, namespace, pod.Name, []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "delay", fmt.Sprintf("%dms", latencyMs)}); err != nil {
			return nil, fmt.Errorf("inject latency on %s: %w", pod.Name, err)
		}
	}
	log.Printf("Injected %dms latency on %d pods in %s", latencyMs, len(podNames), namespace)

	rollback := func() (map[string]any, error) {
		rbCtx := context.Background()
		for _, pod := range pods.Items {
			if _, err := e.execInPod(rbCtx, namespace, pod.Name, []string{"tc", "qdisc", "del", "dev", "eth0", "root"}); err != nil {
				log.Printf("Rollback: remove latency from %s failed: %v", pod.Name, err)
			}
		}
		return map[string]any{"removed_latency": len(podNames)}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "network_latency", "pods": podNames, "latency_ms": latencyMs},
		RollbackFn: rollback,
	}, nil
}

// NetworkLoss injects network packet loss
func (e *K8sEngine) NetworkLoss(ctx context.Context, namespace, labelSelector string, lossPercent int, cfg *domain.ExperimentConfig) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	podNames := podNameList(pods)

	if cfg != nil && cfg.Safety.DryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "network_loss", "pods": podNames, "loss_percent": lossPercent, "dry_run": true},
		}, nil
	}

	for _, pod := range pods.Items {
		if _, err := e.execInPod(ctx, namespace, pod.Name, []string{"tc", "qdisc", "add", "dev", "eth0", "root", "netem", "loss", fmt.Sprintf("%d%%", lossPercent)}); err != nil {
			return nil, fmt.Errorf("inject loss on %s: %w", pod.Name, err)
		}
	}
	log.Printf("Injected %d%% packet loss on %d pods in %s", lossPercent, len(podNames), namespace)

	rollback := func() (map[string]any, error) {
		rbCtx := context.Background()
		for _, pod := range pods.Items {
			if _, err := e.execInPod(rbCtx, namespace, pod.Name, []string{"tc", "qdisc", "del", "dev", "eth0", "root"}); err != nil {
				log.Printf("Rollback: remove loss from %s failed: %v", pod.Name, err)
			}
		}
		return map[string]any{"removed_loss": len(podNames)}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "network_loss", "pods": podNames, "loss_percent": lossPercent},
		RollbackFn: rollback,
	}, nil
}

// CPUStress injects CPU stress via stress-ng
func (e *K8sEngine) CPUStress(ctx context.Context, namespace, labelSelector string, cores, durationSec int, cfg *domain.ExperimentConfig) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	podNames := podNameList(pods)

	if cfg != nil && cfg.Safety.DryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "cpu_stress", "pods": podNames, "cores": cores, "dry_run": true},
		}, nil
	}

	for _, pod := range pods.Items {
		if _, err := e.execInPod(ctx, namespace, pod.Name, []string{
			"stress-ng", "--cpu", fmt.Sprintf("%d", cores),
			"--timeout", fmt.Sprintf("%ds", durationSec), "--quiet",
		}); err != nil {
			return nil, fmt.Errorf("cpu stress on %s: %w", pod.Name, err)
		}
	}
	log.Printf("CPU stress on %d pods in %s", len(podNames), namespace)

	rollback := func() (map[string]any, error) {
		rbCtx := context.Background()
		for _, pod := range pods.Items {
			if _, err := e.execInPod(rbCtx, namespace, pod.Name, []string{"pkill", "-f", "stress-ng"}); err != nil {
				log.Printf("Rollback: kill stress on %s failed: %v", pod.Name, err)
			}
		}
		return map[string]any{"killed_stress": len(podNames)}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "cpu_stress", "pods": podNames, "cores": cores},
		RollbackFn: rollback,
	}, nil
}

// MemoryStress injects memory stress via stress-ng
func (e *K8sEngine) MemoryStress(ctx context.Context, namespace, labelSelector string, memoryBytes string, durationSec int, cfg *domain.ExperimentConfig) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	podNames := podNameList(pods)

	if cfg != nil && cfg.Safety.DryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "memory_stress", "pods": podNames, "memory_bytes": memoryBytes, "dry_run": true},
		}, nil
	}

	for _, pod := range pods.Items {
		if _, err := e.execInPod(ctx, namespace, pod.Name, []string{
			"stress-ng", "--vm", "1", "--vm-bytes", memoryBytes,
			"--timeout", fmt.Sprintf("%ds", durationSec), "--quiet",
		}); err != nil {
			return nil, fmt.Errorf("memory stress on %s: %w", pod.Name, err)
		}
	}
	log.Printf("Memory stress on %d pods in %s", len(podNames), namespace)

	rollback := func() (map[string]any, error) {
		rbCtx := context.Background()
		for _, pod := range pods.Items {
			if _, err := e.execInPod(rbCtx, namespace, pod.Name, []string{"pkill", "-f", "stress-ng"}); err != nil {
				log.Printf("Rollback: kill stress on %s failed: %v", pod.Name, err)
			}
		}
		return map[string]any{"killed_stress": len(podNames)}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "memory_stress", "pods": podNames, "memory_bytes": memoryBytes},
		RollbackFn: rollback,
	}, nil
}

// GetTopology discovers K8s resource topology
func (e *K8sEngine) GetTopology(ctx context.Context, namespace string) (*domain.InfraTopology, error) {
	nodes := make([]domain.TopologyNode, 0)
	edges := make([]domain.TopologyEdge, 0)

	// Deployments
	deployments, err := e.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	for _, dep := range deployments.Items {
		depID := "deploy/" + dep.Name
		health := domain.HealthDegraded
		if dep.Status.ReadyReplicas == dep.Status.Replicas {
			health = domain.HealthHealthy
		}
		nodes = append(nodes, domain.TopologyNode{
			ID:           depID,
			Name:         dep.Name,
			ResourceType: domain.ResourceDeployment,
			Namespace:    &namespace,
			Labels:       dep.Labels,
			Health:       health,
		})
	}

	// ReplicaSets - build RS-to-Deployment ownership map
	replicaSets, err := e.clientset.AppsV1().ReplicaSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list replicasets: %w", err)
	}
	rsToDeployment := make(map[string]string) // RS name -> Deployment name
	for _, rs := range replicaSets.Items {
		for _, owner := range rs.OwnerReferences {
			if owner.Kind == "Deployment" {
				rsToDeployment[rs.Name] = owner.Name
			}
		}
	}

	// Pods
	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	for _, pod := range pods.Items {
		podID := "pod/" + pod.Name
		health := domain.HealthUnknown
		switch pod.Status.Phase {
		case corev1.PodRunning:
			health = domain.HealthHealthy
		case corev1.PodFailed:
			health = domain.HealthUnhealthy
		}
		nodes = append(nodes, domain.TopologyNode{
			ID:           podID,
			Name:         pod.Name,
			ResourceType: domain.ResourcePod,
			Namespace:    &namespace,
			Labels:       pod.Labels,
			Health:       health,
		})

		// Link pod to owner deployment via ReplicaSet ownership chain
		for _, owner := range pod.OwnerReferences {
			if owner.Kind == "ReplicaSet" {
				if depName, ok := rsToDeployment[owner.Name]; ok {
					edges = append(edges, domain.TopologyEdge{
						Source:   "deploy/" + depName,
						Target:   podID,
						Relation: "manages",
					})
				}
			}
		}
	}

	// Services
	services, err := e.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	for _, svc := range services.Items {
		svcID := "svc/" + svc.Name
		nodes = append(nodes, domain.TopologyNode{
			ID:           svcID,
			Name:         svc.Name,
			ResourceType: domain.ResourceService,
			Namespace:    &namespace,
			Labels:       svc.Labels,
			Health:       domain.HealthHealthy,
		})
	}

	return &domain.InfraTopology{Nodes: nodes, Edges: edges}, nil
}

// GetSteadyState captures current steady state metrics
func (e *K8sEngine) GetSteadyState(ctx context.Context, namespace string) (map[string]any, error) {
	pods, err := e.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	running := 0
	for _, p := range pods.Items {
		if p.Status.Phase == corev1.PodRunning {
			running++
		}
	}
	total := len(pods.Items)
	ratio := 1.0
	if total > 0 {
		ratio = float64(running) / float64(total)
	}

	return map[string]any{
		"namespace":          namespace,
		"pods_total":         total,
		"pods_running":       running,
		"pods_healthy_ratio": ratio,
	}, nil
}

func (e *K8sEngine) execInPod(ctx context.Context, namespace, podName string, command []string) (string, error) {
	req := e.clientset.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command,
			Stdout:  true,
			Stderr:  true,
		}, scheme.ParameterCodec)

	executor, err := remotecommand.NewSPDYExecutor(e.restConfig, "POST", req.URL())
	if err != nil {
		return "", fmt.Errorf("exec setup for %s: %w", podName, err)
	}

	var stdout, stderr strings.Builder
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	}); err != nil {
		return stdout.String(), fmt.Errorf("exec in %s: %w (stderr: %s)", podName, err, stderr.String())
	}
	return stdout.String(), nil
}

func podNameList(pods *corev1.PodList) []string {
	names := make([]string, 0, len(pods.Items))
	for _, p := range pods.Items {
		names = append(names, p.Name)
	}
	return names
}

func podNameListFromPods(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, p := range pods {
		names = append(names, p.Name)
	}
	return names
}

func buildPodRollback(clientset kubernetes.Interface, namespace string, pods []corev1.Pod) domain.RollbackFunc {
	return func() (map[string]any, error) {
		rbCtx := context.Background()
		for _, pod := range pods {
			pod.ResourceVersion = ""
			pod.Status = corev1.PodStatus{}
			pod.UID = ""
			if _, err := clientset.CoreV1().Pods(namespace).Create(rbCtx, &pod, metav1.CreateOptions{}); err != nil {
				log.Printf("Rollback: failed to recreate pod %s: %v", pod.Name, err)
			}
		}
		log.Printf("Rollback: recreated %d pods in %s", len(pods), namespace)
		return map[string]any{"recreated": len(pods)}, nil
	}
}
