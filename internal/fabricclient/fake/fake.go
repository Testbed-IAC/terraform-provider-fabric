package fake

import (
	"context"

	"github.com/Testbed-IAC/terraform-provider-fabric/internal/fabricclient"
)

type Client struct {
	CreateFn   func(context.Context, string, string, []string, fabricclient.CreateOpts) ([]fabricclient.Sliver, error)
	GetFn      func(context.Context, string) (*fabricclient.Slice, error)
	ListFn     func(context.Context, string, []string) ([]fabricclient.Slice, error)
	ModifyFn   func(context.Context, string, string) ([]fabricclient.Sliver, error)
	AcceptFn   func(context.Context, string) (*fabricclient.Slice, error)
	RenewFn    func(context.Context, string, string) error
	DeleteFn   func(context.Context, string) error
	SliversFn  func(context.Context, string) ([]fabricclient.Sliver, error)
	ResourceFn func(context.Context, int32, bool) (string, error)
	Calls      []string
}

func (c *Client) CreateSlice(ctx context.Context, name, graphML string, sshKeys []string, opts fabricclient.CreateOpts) ([]fabricclient.Sliver, error) {
	c.Calls = append(c.Calls, "CreateSlice:"+name)
	if c.CreateFn == nil {
		return nil, nil
	}
	return c.CreateFn(ctx, name, graphML, sshKeys, opts)
}

func (c *Client) GetSlice(ctx context.Context, sliceID string) (*fabricclient.Slice, error) {
	c.Calls = append(c.Calls, "GetSlice:"+sliceID)
	if c.GetFn == nil {
		return nil, nil
	}
	return c.GetFn(ctx, sliceID)
}

func (c *Client) ListSlices(ctx context.Context, name string, states []string) ([]fabricclient.Slice, error) {
	c.Calls = append(c.Calls, "ListSlices:"+name)
	if c.ListFn == nil {
		return nil, nil
	}
	return c.ListFn(ctx, name, states)
}

func (c *Client) ModifySlice(ctx context.Context, sliceID, graphML string) ([]fabricclient.Sliver, error) {
	c.Calls = append(c.Calls, "ModifySlice:"+sliceID)
	if c.ModifyFn == nil {
		return nil, nil
	}
	return c.ModifyFn(ctx, sliceID, graphML)
}

func (c *Client) AcceptModify(ctx context.Context, sliceID string) (*fabricclient.Slice, error) {
	c.Calls = append(c.Calls, "AcceptModify:"+sliceID)
	if c.AcceptFn == nil {
		return nil, nil
	}
	return c.AcceptFn(ctx, sliceID)
}

func (c *Client) RenewSlice(ctx context.Context, sliceID, leaseEndTime string) error {
	c.Calls = append(c.Calls, "RenewSlice:"+sliceID)
	if c.RenewFn == nil {
		return nil
	}
	return c.RenewFn(ctx, sliceID, leaseEndTime)
}

func (c *Client) DeleteSlice(ctx context.Context, sliceID string) error {
	c.Calls = append(c.Calls, "DeleteSlice:"+sliceID)
	if c.DeleteFn == nil {
		return nil
	}
	return c.DeleteFn(ctx, sliceID)
}

func (c *Client) GetSlivers(ctx context.Context, sliceID string) ([]fabricclient.Sliver, error) {
	c.Calls = append(c.Calls, "GetSlivers:"+sliceID)
	if c.SliversFn == nil {
		return nil, nil
	}
	return c.SliversFn(ctx, sliceID)
}

func (c *Client) GetResources(ctx context.Context, level int32, forceRefresh bool) (string, error) {
	c.Calls = append(c.Calls, "GetResources")
	if c.ResourceFn == nil {
		return "", nil
	}
	return c.ResourceFn(ctx, level, forceRefresh)
}
