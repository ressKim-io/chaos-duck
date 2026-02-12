package engine

import (
	"context"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/chaosduck/backend-go/internal/domain"
	"github.com/chaosduck/backend-go/internal/safety"
)

// AwsEngine implements chaos operations against AWS resources.
// All mutation methods return (result, rollbackFn).
type AwsEngine struct {
	ec2Client *ec2.Client
	rdsClient *rds.Client
	esm       *safety.EmergencyStopManager
}

// NewAwsEngine creates an AwsEngine with the specified region
func NewAwsEngine(ctx context.Context, region string, esm *safety.EmergencyStopManager) (*AwsEngine, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}

	return &AwsEngine{
		ec2Client: ec2.NewFromConfig(cfg),
		rdsClient: rds.NewFromConfig(cfg),
		esm:       esm,
	}, nil
}

func (e *AwsEngine) checkEmergencyStop() error {
	return e.esm.CheckEmergencyStop()
}

// StopEC2 stops EC2 instances
func (e *AwsEngine) StopEC2(ctx context.Context, instanceIDs []string, dryRun bool) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	if dryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "stop_ec2", "instance_ids": instanceIDs, "dry_run": true},
		}, nil
	}

	_, err := e.ec2Client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: instanceIDs,
	})
	if err != nil {
		return nil, fmt.Errorf("stop EC2 instances: %w", err)
	}
	log.Printf("Stopped EC2 instances: %v", instanceIDs)

	rollback := func() (map[string]any, error) {
		rbCtx := context.Background()
		_, err := e.ec2Client.StartInstances(rbCtx, &ec2.StartInstancesInput{
			InstanceIds: instanceIDs,
		})
		if err != nil {
			return nil, fmt.Errorf("start EC2 instances: %w", err)
		}
		log.Printf("Rollback: started EC2 instances: %v", instanceIDs)
		return map[string]any{"started": instanceIDs}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "stop_ec2", "instance_ids": instanceIDs},
		RollbackFn: rollback,
	}, nil
}

// FailoverRDS forces an RDS cluster failover
func (e *AwsEngine) FailoverRDS(ctx context.Context, dbClusterID string, dryRun bool) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	if dryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "rds_failover", "db_cluster_id": dbClusterID, "dry_run": true},
		}, nil
	}

	_, err := e.rdsClient.FailoverDBCluster(ctx, &rds.FailoverDBClusterInput{
		DBClusterIdentifier: aws.String(dbClusterID),
	})
	if err != nil {
		return nil, fmt.Errorf("failover RDS: %w", err)
	}
	log.Printf("Triggered RDS failover: %s", dbClusterID)

	// RDS failover is self-healing
	rollback := func() (map[string]any, error) {
		log.Printf("RDS failover rollback: cluster will self-heal")
		return map[string]any{"note": "RDS failover is self-healing"}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "rds_failover", "db_cluster_id": dbClusterID},
		RollbackFn: rollback,
	}, nil
}

// BlackholeRoute creates a blackhole route in a VPC route table
func (e *AwsEngine) BlackholeRoute(ctx context.Context, routeTableID, destCIDR string, dryRun bool) (*domain.ChaosResult, error) {
	if err := e.checkEmergencyStop(); err != nil {
		return nil, err
	}

	if dryRun {
		return &domain.ChaosResult{
			Result: map[string]any{"action": "route_blackhole", "route_table_id": routeTableID, "destination_cidr": destCIDR, "dry_run": true},
		}, nil
	}

	// Save existing route for rollback
	tables, err := e.ec2Client.DescribeRouteTables(ctx, &ec2.DescribeRouteTablesInput{
		RouteTableIds: []string{routeTableID},
	})
	if err != nil {
		return nil, fmt.Errorf("describe route tables: %w", err)
	}

	var originalGateway *string
	if len(tables.RouteTables) > 0 {
		for _, route := range tables.RouteTables[0].Routes {
			if route.DestinationCidrBlock != nil && *route.DestinationCidrBlock == destCIDR {
				originalGateway = route.GatewayId
				break
			}
		}
	}

	// Use ReplaceRoute if route exists, CreateRoute otherwise
	if originalGateway != nil {
		_, err = e.ec2Client.ReplaceRoute(ctx, &ec2.ReplaceRouteInput{
			RouteTableId:         aws.String(routeTableID),
			DestinationCidrBlock: aws.String(destCIDR),
		})
	} else {
		_, err = e.ec2Client.CreateRoute(ctx, &ec2.CreateRouteInput{
			RouteTableId:         aws.String(routeTableID),
			DestinationCidrBlock: aws.String(destCIDR),
		})
	}
	if err != nil {
		return nil, fmt.Errorf("create blackhole route: %w", err)
	}
	log.Printf("Created blackhole route: %s -> %s", routeTableID, destCIDR)

	rollback := func() (map[string]any, error) {
		rbCtx := context.Background()
		_, err := e.ec2Client.DeleteRoute(rbCtx, &ec2.DeleteRouteInput{
			RouteTableId:         aws.String(routeTableID),
			DestinationCidrBlock: aws.String(destCIDR),
		})
		if err != nil {
			return nil, fmt.Errorf("delete route: %w", err)
		}
		if originalGateway != nil {
			_, err := e.ec2Client.CreateRoute(rbCtx, &ec2.CreateRouteInput{
				RouteTableId:         aws.String(routeTableID),
				DestinationCidrBlock: aws.String(destCIDR),
				GatewayId:            originalGateway,
			})
			if err != nil {
				return nil, fmt.Errorf("restore route: %w", err)
			}
		}
		log.Printf("Rollback: restored route %s", destCIDR)
		return map[string]any{"restored": destCIDR}, nil
	}

	return &domain.ChaosResult{
		Result:     map[string]any{"action": "route_blackhole", "route_table_id": routeTableID, "destination_cidr": destCIDR},
		RollbackFn: rollback,
	}, nil
}

// GetTopology discovers AWS resource topology
func (e *AwsEngine) GetTopology(ctx context.Context) (*domain.InfraTopology, error) {
	nodes := make([]domain.TopologyNode, 0)
	edges := make([]domain.TopologyEdge, 0)

	// EC2 instances
	reservations, err := e.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("describe EC2 instances: %w", err)
	}

	for _, res := range reservations.Reservations {
		for _, inst := range res.Instances {
			instID := aws.ToString(inst.InstanceId)
			tags := make(map[string]string)
			instName := instID
			for _, t := range inst.Tags {
				tags[aws.ToString(t.Key)] = aws.ToString(t.Value)
				if aws.ToString(t.Key) == "Name" {
					instName = aws.ToString(t.Value)
				}
			}

			health := domain.HealthUnknown
			stateName := ""
			if inst.State != nil {
				stateName = string(inst.State.Name)
				switch inst.State.Name {
				case ec2types.InstanceStateNameRunning:
					health = domain.HealthHealthy
				case ec2types.InstanceStateNameStopped:
					health = domain.HealthUnhealthy
				}
			}

			nodes = append(nodes, domain.TopologyNode{
				ID:           instID,
				Name:         instName,
				ResourceType: domain.ResourceEC2,
				Labels:       tags,
				Health:       health,
				Metadata: map[string]any{
					"state": stateName,
					"type":  string(inst.InstanceType),
				},
			})

			if inst.VpcId != nil {
				edges = append(edges, domain.TopologyEdge{
					Source:   aws.ToString(inst.VpcId),
					Target:   instID,
					Relation: "contains",
				})
			}
		}
	}

	// RDS clusters
	clusters, err := e.rdsClient.DescribeDBClusters(ctx, &rds.DescribeDBClustersInput{})
	if err != nil {
		log.Printf("RDS describe failed (non-fatal): %v", err)
	} else {
		for _, cluster := range clusters.DBClusters {
			clusterID := aws.ToString(cluster.DBClusterIdentifier)
			health := domain.HealthDegraded
			if aws.ToString(cluster.Status) == "available" {
				health = domain.HealthHealthy
			}
			nodes = append(nodes, domain.TopologyNode{
				ID:           clusterID,
				Name:         clusterID,
				ResourceType: domain.ResourceRDS,
				Health:       health,
				Metadata: map[string]any{
					"engine": aws.ToString(cluster.Engine),
					"status": aws.ToString(cluster.Status),
				},
			})
		}
	}

	return &domain.InfraTopology{Nodes: nodes, Edges: edges}, nil
}
