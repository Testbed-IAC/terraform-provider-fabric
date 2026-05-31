package fabricclient

import "context"

type Slice struct {
	SliceID        string
	Name           string
	State          string
	GraphID        string
	Model          string
	LeaseStartTime string
	LeaseEndTime   string
	Notice         string
}

type Sliver struct {
	SliceID      string
	SliverID     string
	GraphNodeID  string
	SliverType   string
	State        string
	PendingState string
	JoinState    string
	ManagementIP string
	Notice       string
}

type CreateOpts struct {
	LifetimeHours  int32
	LeaseStartTime string
	LeaseEndTime   string
}

// ResourcesQuery contains filters for FABRIC resource model requests.
type ResourcesQuery struct {
	// Level controls the detail level returned by the orchestrator.
	Level int32
	// ForceRefresh bypasses cached resource data when true.
	ForceRefresh bool
	// StartDate is the FABRIC-formatted availability start time.
	StartDate string
	// EndDate is the FABRIC-formatted availability end time.
	EndDate string
	// Includes is a comma-separated list of site codes to include.
	Includes string
	// Excludes is a comma-separated list of site codes to exclude.
	Excludes string
	// GraphFormat is required for portal resource requests.
	GraphFormat string
}

// POAVCPUCPU maps one guest vCPU to one host CPU for POA requests.
type POAVCPUCPU struct {
	VCPU string
	CPU  string
}

// POAKey contains one SSH public key entry for POA key operations.
type POAKey struct {
	Key     string
	Comment string
}

// POARequest contains a FABRIC perform-operational-action request.
type POARequest struct {
	Operation  string
	VCPUCPUMap []POAVCPUCPU
	NodeSet    []string
	BDF        []string
	Keys       []POAKey
}

// POA contains a FABRIC perform-operational-action status response.
type POA struct {
	POAID     string
	Operation string
	State     string
	SliverID  string
	SliceID   string
	Error     string
	InfoJSON  string
}

// MetricsQuery contains filters for FABRIC metrics overview requests.
type MetricsQuery struct {
	ExcludedProjects []string
}

type FabricClient interface {
	CreateSlice(ctx context.Context, name, graphML string, sshKeys []string, opts CreateOpts) ([]Sliver, error)
	GetSlice(ctx context.Context, sliceID string) (*Slice, error)
	ListSlices(ctx context.Context, name string, states []string) ([]Slice, error)
	ModifySlice(ctx context.Context, sliceID, graphML string) ([]Sliver, error)
	AcceptModify(ctx context.Context, sliceID string) (*Slice, error)
	RenewSlice(ctx context.Context, sliceID, leaseEndTime string) error
	DeleteSlice(ctx context.Context, sliceID string) error
	GetSlivers(ctx context.Context, sliceID string) ([]Sliver, error)
	GetResources(ctx context.Context, query ResourcesQuery) (string, error)
	GetPortalResources(ctx context.Context, query ResourcesQuery) (string, error)
	CreatePOA(ctx context.Context, sliverID string, request POARequest) (*POA, error)
	GetPOA(ctx context.Context, poaID string) (*POA, error)
	GetMetricsOverview(ctx context.Context, query MetricsQuery) (string, error)
}
