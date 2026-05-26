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
	SliceID     string
	SliverID    string
	GraphNodeID string
	SliverType  string
	State       string
	Notice      string
}

type CreateOpts struct {
	LifetimeHours  int32
	LeaseStartTime string
	LeaseEndTime   string
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
	GetResources(ctx context.Context, level int32, forceRefresh bool) (string, error)
}
