package handler

import (
	"net/http"

	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/engine"
	"github.com/gin-gonic/gin"
)

// TopologyHandler handles topology discovery endpoints
type TopologyHandler struct {
	k8s *engine.K8sEngine
	aws *engine.AwsEngine
}

// NewTopologyHandler creates a new TopologyHandler
func NewTopologyHandler(k8s *engine.K8sEngine, aws *engine.AwsEngine) *TopologyHandler {
	return &TopologyHandler{k8s: k8s, aws: aws}
}

// GetK8sTopology returns Kubernetes resource topology
func (h *TopologyHandler) GetK8sTopology(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.k8s == nil {
		c.JSON(http.StatusOK, domain.InfraTopology{Nodes: []domain.TopologyNode{}, Edges: []domain.TopologyEdge{}})
		return
	}

	topo, err := h.k8s.GetTopology(c.Request.Context(), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, topo)
}

// GetAWSTopology returns AWS resource topology
func (h *TopologyHandler) GetAWSTopology(c *gin.Context) {
	if h.aws == nil {
		c.JSON(http.StatusOK, domain.InfraTopology{Nodes: []domain.TopologyNode{}, Edges: []domain.TopologyEdge{}})
		return
	}

	topo, err := h.aws.GetTopology(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, topo)
}

// GetCombinedTopology returns combined K8s + AWS topology
func (h *TopologyHandler) GetCombinedTopology(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	combined := domain.InfraTopology{
		Nodes: make([]domain.TopologyNode, 0),
		Edges: make([]domain.TopologyEdge, 0),
	}

	if h.k8s != nil {
		k8sTopo, err := h.k8s.GetTopology(c.Request.Context(), namespace)
		if err == nil {
			combined.Nodes = append(combined.Nodes, k8sTopo.Nodes...)
			combined.Edges = append(combined.Edges, k8sTopo.Edges...)
		}
	}

	if h.aws != nil {
		awsTopo, err := h.aws.GetTopology(c.Request.Context())
		if err == nil {
			combined.Nodes = append(combined.Nodes, awsTopo.Nodes...)
			combined.Edges = append(combined.Edges, awsTopo.Edges...)
		}
	}

	c.JSON(http.StatusOK, combined)
}

// GetSteadyState returns current steady state metrics
func (h *TopologyHandler) GetSteadyState(c *gin.Context) {
	namespace := c.DefaultQuery("namespace", "default")

	if h.k8s == nil {
		c.JSON(http.StatusOK, gin.H{
			"namespace":          namespace,
			"pods_total":         0,
			"pods_running":       0,
			"pods_healthy_ratio": 1.0,
		})
		return
	}

	state, err := h.k8s.GetSteadyState(c.Request.Context(), namespace)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"detail": err.Error()})
		return
	}
	c.JSON(http.StatusOK, state)
}
