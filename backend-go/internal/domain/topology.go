package domain

// ResourceType identifies the kind of infrastructure resource
type ResourceType string

const (
	ResourcePod        ResourceType = "pod"
	ResourceService    ResourceType = "service"
	ResourceDeployment ResourceType = "deployment"
	ResourceNode       ResourceType = "node"
	ResourceNamespace  ResourceType = "namespace"
	ResourceEC2        ResourceType = "ec2"
	ResourceRDS        ResourceType = "rds"
	ResourceVPC        ResourceType = "vpc"
	ResourceSubnet     ResourceType = "subnet"
)

// HealthStatus describes the health of a resource
type HealthStatus string

const (
	HealthHealthy   HealthStatus = "healthy"
	HealthDegraded  HealthStatus = "degraded"
	HealthUnhealthy HealthStatus = "unhealthy"
	HealthUnknown   HealthStatus = "unknown"
)

// TopologyNode represents a single infrastructure resource
type TopologyNode struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	ResourceType ResourceType      `json:"resource_type"`
	Namespace    *string           `json:"namespace,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
	Health       HealthStatus      `json:"health"`
	Metadata     map[string]any    `json:"metadata,omitempty"`
}

// TopologyEdge represents a relationship between two resources
type TopologyEdge struct {
	Source   string         `json:"source"`
	Target   string         `json:"target"`
	Relation string         `json:"relation"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// InfraTopology is the full resource graph
type InfraTopology struct {
	Nodes     []TopologyNode `json:"nodes"`
	Edges     []TopologyEdge `json:"edges"`
	Timestamp *string        `json:"timestamp,omitempty"`
}

// ResilienceScore summarizes system resilience
type ResilienceScore struct {
	Overall         float64            `json:"overall"`
	Categories      map[string]float64 `json:"categories,omitempty"`
	Recommendations []string           `json:"recommendations,omitempty"`
	Details         *string            `json:"details,omitempty"`
}
