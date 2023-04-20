package smtp

import (
	"context"
	"os"
	"net/smtp"


	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces
var (
	_ provider.Provider = &smtpProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New() provider.Provider {
	return &smtpProvider{}
}

// smtpProvider is the provider implementation.
type smtpProvider struct{}

type client struct {
	auth           smtp.Auth
	host, username string
	port           string
}

// smtpProviderModel maps provider schema data to a Go type.
type smtpProviderModel struct {
	Host     types.String `tfsdk:"host"`
	// TODO: Convert the port to number
	Port     types.String `tfsdk:"port"`
	Username types.String `tfsdk:"username"`
	Password types.String `tfsdk:"password"`
}

// Metadata returns the provider type name.
func (p *smtpProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "smtp"
}

// Schema defines the provider-level schema for configuration data.
func (p *smtpProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Interact with SMTP.",
		Attributes: map[string]schema.Attribute{
			"host": schema.StringAttribute{
				Optional: true,
				Description: "SMTP host domain. eg. smtp.example.com. May also be provided via SMTP_HOST environment variable.",
			},
			"port": schema.StringAttribute{
				Optional: true,
				Description: "SMTP host port. eg: 25. May also be provided via SMTP_PORT environment variable.",
			},
			"username": schema.StringAttribute{
				Optional: true,
				Description: "User name to authenticate with SMTP. May also be provided via SMTP_USERNAME environment variable.",
			},
			"password": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				Description: "Password to authenticate with SMTP. May also be provided via SMTP_PASSWORD environment variable.",
			},
		},
	}
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *smtpProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring SMTP client")

	// Retrieve provider data from configuration
	var config smtpProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If practitioner provided a configuration value for any of the
	// attributes, it must be a known value.

	if config.Host.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Unknown SMTP Host",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the HashiCups API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_HOST environment variable.",
		)
	}

	if config.Port.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("port"),
			"Unknown SMTP port",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the HashiCups API host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_HOST environment variable.",
		)
	}

	if config.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Unknown HashiCups API Username",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the HashiCups API username. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_USERNAME environment variable.",
		)
	}

	if config.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Unknown HashiCups API Password",
			"The provider cannot create the HashiCups API client as there is an unknown configuration value for the HashiCups API password. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the HASHICUPS_PASSWORD environment variable.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Default values to environment variables, but override
	// with Terraform configuration value if set.

	host := os.Getenv("SMTP_HOST")
	port := os.Getenv("SMTP_PORT")
	username := os.Getenv("SMTP_USERNAME")
	password := os.Getenv("SMTP_PASSWORD")
	

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}
	if !config.Port.IsNull() {
		port = config.Port.ValueString()
	}
	if !config.Username.IsNull() {
		username = config.Username.ValueString()
	}

	if !config.Password.IsNull() {
		password = config.Password.ValueString()
	}

	// If any of the expected configurations are missing, return
	// errors with provider-specific guidance.

	if host == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("host"),
			"Missing HashiCups API Host",
			"The provider cannot create the HashiCups API client as there is a missing or empty value for the HashiCups API host. "+
				"Set the host value in the configuration or use the HASHICUPS_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}
	if port == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("port"),
			"Missing SMTP host port",
			"The provider cannot create the HashiCups API client as there is a missing or empty value for the HashiCups API host. "+
				"Set the host value in the configuration or use the HASHICUPS_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}
	if username == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing HashiCups API Username",
			"The provider cannot create the HashiCups API client as there is a missing or empty value for the HashiCups API username. "+
				"Set the username value in the configuration or use the HASHICUPS_USERNAME environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if password == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing HashiCups API Password",
			"The provider cannot create the HashiCups API client as there is a missing or empty value for the HashiCups API password. "+
				"Set the password value in the configuration or use the HASHICUPS_PASSWORD environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "smtp_host", host)
	ctx = tflog.SetField(ctx, "smtp_port", port)
	ctx = tflog.SetField(ctx, "smtp_username", username)
	ctx = tflog.SetField(ctx, "smtp_password", password)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "smtp_password")

	tflog.Debug(ctx, "Creating HashiCups client")

	// Create a new HashiCups client using the configuration values
	auth := smtp.PlainAuth("", username, password, host)

	client := &client{
		host:     host,
		port:     port,
		username: username,
		auth: auth,
	}

	// Make the HashiCups client available during DataSource and Resource
	// type Configure methods.
	resp.DataSourceData = client
	resp.ResourceData = client

	tflog.Info(ctx, "Configured SMTP client", map[string]any{"success": true})
}

// DataSources defines the data sources implemented in the provider.
func (p *smtpProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

// Resources defines the resources implemented in the provider.
func (p *smtpProvider) Resources(_ context.Context) []func() resource.Resource {
    return []func() resource.Resource{
        NewSendMailResource,
    }
}
