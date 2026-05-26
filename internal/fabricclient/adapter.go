package fabricclient

import (
	"context"
	"fmt"

	openapi "github.com/Testbed-IAC/fabric-orchestrator-go-client"
)

type Adapter struct {
	api   *openapi.APIClient
	token string
}

func New(orchestratorURL, token string) *Adapter {
	cfg := openapi.NewConfiguration()
	if orchestratorURL != "" {
		cfg.Servers = openapi.ServerConfigurations{{URL: orchestratorURL}}
	}
	return &Adapter{api: openapi.NewAPIClient(cfg), token: token}
}

func (a *Adapter) authCtx(ctx context.Context) context.Context {
	return context.WithValue(ctx, openapi.ContextAccessToken, a.token)
}

func (a *Adapter) CreateSlice(ctx context.Context, name, graphML string, sshKeys []string, opts CreateOpts) ([]Sliver, error) {
	body := openapi.NewSlicesPost(graphML, sshKeys)
	req := a.api.SlicesAPI.SlicesCreatesPost(a.authCtx(ctx)).Name(name).SlicesPost(*body)
	if opts.LifetimeHours > 0 {
		req = req.Lifetime(opts.LifetimeHours)
	}
	if opts.LeaseStartTime != "" {
		req = req.LeaseStartTime(opts.LeaseStartTime)
	}
	if opts.LeaseEndTime != "" {
		req = req.LeaseEndTime(opts.LeaseEndTime)
	}
	resp, httpResp, err := req.Execute()
	if err != nil {
		return nil, fmt.Errorf("creating slice: %w", mapHTTPErr(httpResp, err))
	}
	return convertSlivers(resp.Data), nil
}

func (a *Adapter) GetSlice(ctx context.Context, sliceID string) (*Slice, error) {
	resp, httpResp, err := a.api.SlicesAPI.SlicesSliceIdGet(a.authCtx(ctx), sliceID).GraphFormat("GRAPHML").Execute()
	if err != nil {
		return nil, fmt.Errorf("getting slice: %w", mapHTTPErr(httpResp, err))
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, ErrNotFound
	}
	s := resp.Data[0]
	return &Slice{
		SliceID:        s.GetSliceId(),
		Name:           s.GetName(),
		State:          s.GetState(),
		GraphID:        s.GetGraphId(),
		Model:          s.GetModel(),
		LeaseStartTime: s.GetLeaseStartTime(),
		LeaseEndTime:   s.GetLeaseEndTime(),
	}, nil
}

func (a *Adapter) ListSlices(ctx context.Context, name string, states []string) ([]Slice, error) {
	out := []Slice{}
	var offset int32
	for {
		req := a.api.SlicesAPI.SlicesGet(a.authCtx(ctx)).Limit(200).Offset(offset)
		if name != "" {
			req = req.Name(name).ExactMatch(true)
		}
		if len(states) > 0 {
			req = req.States(states)
		}
		page, httpResp, err := req.Execute()
		if err != nil {
			return nil, fmt.Errorf("listing slices: %w", mapHTTPErr(httpResp, err))
		}
		for _, s := range page.Data {
			out = append(out, Slice{SliceID: s.GetSliceId(), Name: s.GetName(), GraphID: s.GetGraphId(), State: s.GetState()})
		}
		if len(page.Data) < 200 {
			return out, nil
		}
		offset += 200
	}
}

func (a *Adapter) ModifySlice(ctx context.Context, sliceID, graphML string) ([]Sliver, error) {
	resp, httpResp, err := a.api.SlicesAPI.SlicesModifySliceIdPut(a.authCtx(ctx), sliceID).Body(graphML).Execute()
	if err != nil {
		return nil, fmt.Errorf("modifying slice: %w", mapHTTPErr(httpResp, err))
	}
	return convertSlivers(resp.Data), nil
}

func (a *Adapter) AcceptModify(ctx context.Context, sliceID string) (*Slice, error) {
	resp, httpResp, err := a.api.SlicesAPI.SlicesModifySliceIdAcceptPost(a.authCtx(ctx), sliceID).Execute()
	if err != nil {
		return nil, fmt.Errorf("accepting modify: %w", mapHTTPErr(httpResp, err))
	}
	if resp == nil || len(resp.Data) == 0 {
		return nil, ErrNotFound
	}
	s := resp.Data[0]
	return &Slice{SliceID: s.GetSliceId(), Name: s.GetName(), State: s.GetState(), GraphID: s.GetGraphId(), Model: s.GetModel()}, nil
}

func (a *Adapter) RenewSlice(ctx context.Context, sliceID, leaseEndTime string) error {
	_, httpResp, err := a.api.SlicesAPI.SlicesRenewSliceIdPost(a.authCtx(ctx), sliceID).LeaseEndTime(leaseEndTime).Execute()
	if err != nil {
		return fmt.Errorf("renewing slice: %w", mapHTTPErr(httpResp, err))
	}
	return nil
}

func (a *Adapter) DeleteSlice(ctx context.Context, sliceID string) error {
	_, httpResp, err := a.api.SlicesAPI.SlicesDeleteSliceIdDelete(a.authCtx(ctx), sliceID).Execute()
	if err != nil {
		return fmt.Errorf("deleting slice: %w", mapHTTPErr(httpResp, err))
	}
	return nil
}

func (a *Adapter) GetSlivers(ctx context.Context, sliceID string) ([]Sliver, error) {
	resp, httpResp, err := a.api.SliversAPI.SliversGet(a.authCtx(ctx)).SliceId(sliceID).Execute()
	if err != nil {
		return nil, fmt.Errorf("getting slivers: %w", mapHTTPErr(httpResp, err))
	}
	return convertSlivers(resp.Data), nil
}

func (a *Adapter) GetResources(ctx context.Context, level int32, forceRefresh bool) (string, error) {
	resp, httpResp, err := a.api.ResourcesAPI.ResourcesGet(a.authCtx(ctx)).Level(level).ForceRefresh(forceRefresh).Execute()
	if err != nil {
		return "", fmt.Errorf("getting resources: %w", mapHTTPErr(httpResp, err))
	}
	if resp == nil || len(resp.Data) == 0 {
		return "", nil
	}
	return resp.Data[0].GetModel(), nil
}

func convertSlivers(in []openapi.Sliver) []Sliver {
	out := make([]Sliver, 0, len(in))
	for _, s := range in {
		out = append(out, Sliver{
			SliceID:     s.GetSliceId(),
			SliverID:    s.GetSliverId(),
			GraphNodeID: s.GetGraphNodeId(),
			SliverType:  s.GetSliverType(),
			State:       s.GetState(),
			Notice:      s.GetNotice(),
		})
	}
	return out
}
