package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTopologyNodeJSON(t *testing.T) {
	ns := "default"
	node := TopologyNode{
		ID:           "pod/nginx-1",
		Name:         "nginx-1",
		ResourceType: ResourcePod,
		Namespace:    &ns,
		Labels:       map[string]string{"app": "nginx"},
		Health:       HealthHealthy,
		Metadata:     map[string]any{"phase": "Running"},
	}

	data, err := json.Marshal(node)
	require.NoError(t, err)

	var decoded TopologyNode
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, node.ID, decoded.ID)
	assert.Equal(t, node.Name, decoded.Name)
	assert.Equal(t, ResourcePod, decoded.ResourceType)
	assert.Equal(t, "default", *decoded.Namespace)
	assert.Equal(t, HealthHealthy, decoded.Health)
}

func TestTopologyEdgeJSON(t *testing.T) {
	edge := TopologyEdge{
		Source:   "deploy/nginx",
		Target:   "pod/nginx-1",
		Relation: "manages",
	}

	data, err := json.Marshal(edge)
	require.NoError(t, err)

	var decoded TopologyEdge
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "deploy/nginx", decoded.Source)
	assert.Equal(t, "pod/nginx-1", decoded.Target)
	assert.Equal(t, "manages", decoded.Relation)
}

func TestInfraTopologyEmpty(t *testing.T) {
	topo := InfraTopology{
		Nodes: []TopologyNode{},
		Edges: []TopologyEdge{},
	}

	data, err := json.Marshal(topo)
	require.NoError(t, err)
	assert.Contains(t, string(data), `"nodes":[]`)
	assert.Contains(t, string(data), `"edges":[]`)
}

func TestResourceTypeValues(t *testing.T) {
	assert.Equal(t, ResourceType("pod"), ResourcePod)
	assert.Equal(t, ResourceType("service"), ResourceService)
	assert.Equal(t, ResourceType("deployment"), ResourceDeployment)
	assert.Equal(t, ResourceType("ec2"), ResourceEC2)
	assert.Equal(t, ResourceType("rds"), ResourceRDS)
}

func TestHealthStatusValues(t *testing.T) {
	assert.Equal(t, HealthStatus("healthy"), HealthHealthy)
	assert.Equal(t, HealthStatus("degraded"), HealthDegraded)
	assert.Equal(t, HealthStatus("unhealthy"), HealthUnhealthy)
	assert.Equal(t, HealthStatus("unknown"), HealthUnknown)
}

func TestResilienceScoreJSON(t *testing.T) {
	details := "Good resilience"
	score := ResilienceScore{
		Overall:         85.5,
		Categories:      map[string]float64{"network": 90.0, "compute": 80.0},
		Recommendations: []string{"Add more replicas"},
		Details:         &details,
	}

	data, err := json.Marshal(score)
	require.NoError(t, err)

	var decoded ResilienceScore
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, 85.5, decoded.Overall)
	assert.Equal(t, 90.0, decoded.Categories["network"])
	assert.Len(t, decoded.Recommendations, 1)
	assert.Equal(t, "Good resilience", *decoded.Details)
}
