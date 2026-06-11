package datasource

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	fabricclient "github.com/Testbed-IAC/fabric-go-fim/pkg/client"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/providercfg"
	"github.com/Testbed-IAC/terraform-provider-fabric/internal/tfutil"
)

// ResourcesDataSource fetches the raw FABRIC advertised-resource model from the
// orchestrator or portal resources endpoint.
type ResourcesDataSource struct {
	client fabricclient.API
}

var errStartDate = errors.New("invalid start date")
var errEndDate = errors.New("invalid end date")

// NewResources returns the FABRIC resources data source.
func NewResources() datasource.DataSource {
	return &ResourcesDataSource{}
}

func (d *ResourcesDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resources"
}

func (d *ResourcesDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description:         "Fetch the FABRIC available resources model from the orchestrator or portal resources endpoint.",
		MarkdownDescription: "Fetch the FABRIC available resources model from the orchestrator or portal resources endpoint. Use this data source when you need the raw advertised topology or portal graph payload for planning and inspection.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Synthetic data source identifier assigned by the provider.", MarkdownDescription: "Synthetic data source identifier assigned by the provider."},
			"level":         schema.Int64Attribute{Optional: true, Computed: true, Description: "Resource detail level to request from FABRIC. Defaults to 1.", MarkdownDescription: "Resource detail level to request from FABRIC. Defaults to `1`."},
			"force_refresh": schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to bypass cached resource information. Defaults to false.", MarkdownDescription: "Whether to bypass cached resource information. Defaults to `false`."},
			"start_date":    schema.StringAttribute{Optional: true, Validators: []validator.String{tfutil.FabricTimeValidator{}}, Description: "Availability start time in FABRIC or RFC3339 format. When omitted, FABRIC uses its default availability window.", MarkdownDescription: "Availability start time in FABRIC or RFC3339 format. When omitted, FABRIC uses its default availability window."},
			"end_date":      schema.StringAttribute{Optional: true, Validators: []validator.String{tfutil.FabricTimeValidator{}}, Description: "Availability end time in FABRIC or RFC3339 format. When omitted, FABRIC uses its default availability window.", MarkdownDescription: "Availability end time in FABRIC or RFC3339 format. When omitted, FABRIC uses its default availability window."},
			"includes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to include, such as RENC,UKY.", MarkdownDescription: "Comma-separated site codes to include, such as `RENC,UKY`."},
			"excludes":      schema.StringAttribute{Optional: true, Description: "Comma-separated site codes to exclude, such as RENC,UKY.", MarkdownDescription: "Comma-separated site codes to exclude, such as `RENC,UKY`."},
			"graph_format":  schema.StringAttribute{Optional: true, Description: "Portal resource graph format. When set, the provider calls the portal resources endpoint instead of the orchestrator resources endpoint.", MarkdownDescription: "Portal resource graph format. When set, the provider calls the portal resources endpoint instead of the orchestrator resources endpoint."},
			"model":         schema.StringAttribute{Computed: true, Description: "Opaque resource model returned by FABRIC and assigned by the provider after lookup.", MarkdownDescription: "Opaque resource model returned by FABRIC and assigned by the provider after lookup."},
		},
	}
}

func (d *ResourcesDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providercfg.Data)
	if !ok {
		resp.Diagnostics.AddError("Unexpected provider data", "Provider data was not configured correctly.")
		return
	}
	d.client = data.Client
}

func (d *ResourcesDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config ResourcesDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	level := tfutil.Int64Value(config.Level)
	if level == 0 {
		level = 1
	}
	query, err := resourcesQuery(config, level)
	if err != nil {
		resp.Diagnostics.AddAttributeError(resourcesQueryErrorPath(err), "Invalid FABRIC resources query", "The resources date filters could not be parsed. Original error: "+err.Error())
		return
	}
	model, err := d.readResourcesModel(ctx, query)
	if err != nil {
		resp.Diagnostics.AddError("Read FABRIC resources failed", err.Error())
		return
	}
	config.ID = types.StringValue("resources")
	config.Level = types.Int64Value(level)
	config.ForceRefresh = types.BoolValue(tfutil.BoolValue(config.ForceRefresh))
	if query.StartDate != "" {
		config.StartDate = types.StringValue(query.StartDate)
	}
	if query.EndDate != "" {
		config.EndDate = types.StringValue(query.EndDate)
	}
	config.Model = types.StringValue(model)
	resp.Diagnostics.Append(resp.State.Set(ctx, &config)...)
}

func (d *ResourcesDataSource) readResourcesModel(ctx context.Context, query fabricclient.ResourcesQuery) (string, error) {
	if query.GraphFormat != "" {
		model, err := d.client.GetPortalResources(ctx, query)
		if err != nil {
			return "", fmt.Errorf("getting portal resources: %w", err)
		}
		return model, nil
	}
	model, err := d.client.GetResources(ctx, query)
	if err != nil {
		return "", fmt.Errorf("getting resources: %w", err)
	}
	return model, nil
}

func resourcesQuery(config ResourcesDataSourceModel, level int64) (fabricclient.ResourcesQuery, error) {
	startDate, err := tfutil.CanonicalFabricTimeString(tfutil.StringValue(config.StartDate))
	if err != nil {
		return fabricclient.ResourcesQuery{}, fmt.Errorf("%w: %w", errStartDate, err)
	}
	endDate, err := tfutil.CanonicalFabricTimeString(tfutil.StringValue(config.EndDate))
	if err != nil {
		return fabricclient.ResourcesQuery{}, fmt.Errorf("%w: %w", errEndDate, err)
	}
	return fabricclient.ResourcesQuery{
		Level:        int32(level),
		ForceRefresh: tfutil.BoolValue(config.ForceRefresh),
		StartDate:    startDate,
		EndDate:      endDate,
		Includes:     tfutil.StringValue(config.Includes),
		Excludes:     tfutil.StringValue(config.Excludes),
		GraphFormat:  tfutil.StringValue(config.GraphFormat),
	}, nil
}

func resourcesQueryErrorPath(err error) path.Path {
	if errors.Is(err, errEndDate) {
		return path.Root("end_date")
	}
	return path.Root("start_date")
}
