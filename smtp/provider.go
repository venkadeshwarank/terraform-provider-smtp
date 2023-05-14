package smtp

import (
	"context"
	"net/smtp"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
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
	Host types.String `tfsdk:"host"`
	// TODO: Convert the port to number
	Port           types.String `tfsdk:"port"`
	Authentication types.Bool   `tfsdk:"authentication"`
	Username       types.String `tfsdk:"username"`
	Password       types.String `tfsdk:"password"`
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
				Optional:    true,
				Description: "SMTP host domain. eg. smtp.example.com. May also be provided via SMTP_HOST environment variable.",
			},
			"port": schema.StringAttribute{
				Optional:    true,
				Description: "SMTP host port. eg: 25. May also be provided via SMTP_PORT environment variable.",
			},
			"authentication": schema.BoolAttribute{
				Optional:    true,
				Description: "Enable or Disable the authentication with SMTP (by default, it sets to 'true'). May also be provided via SMTP_AUTHENTICATION environment variable.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "User name to authenticate with SMTP. May also be provided via SMTP_USERNAME environment variable.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password to authenticate with SMTP. May also be provided via SMTP_PASSWORD environment variable.",
			},
		},
	}
}

// Configure prepares a SMTP client for data sources and resources.
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
			"The provider cannot connect to SMTP as there is an unknown configuration value for the SMTP host. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SMTP_HOST environment variable.",
		)
	}

	if config.Port.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("port"),
			"Unknown SMTP port",
			"The provider cannot create the SMTP client as there is an unknown configuration value for the SMTP port. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SMTP_PASSWORD environment variable.",
		)
	}

	if config.Authentication.IsUnknown() {
		config.Authentication = basetypes.NewBoolValue(true)
	}

	if config.Authentication.ValueBool() && config.Username.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Unknown SMTP Username",
			"The provider cannot create the SMTP client as there is an unknown configuration value for the SMTP username. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SMTP_USERNAME environment variable.",
		)
	}

	if config.Authentication.ValueBool() && config.Password.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Unknown SMTP Password",
			"The provider cannot create the SMTP client as there is an unknown configuration value for the SMTP password. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the SMTP_PASSWORD environment variable.",
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

	authentication, err := strconv.ParseBool(os.Getenv("SMTP_AUTHENTICATION"))
	if err != nil {
		authentication = true
	}

	if !config.Host.IsNull() {
		host = config.Host.ValueString()
	}
	if !config.Port.IsNull() {
		port = config.Port.ValueString()
	}

	if !config.Authentication.IsNull() {
		authentication = config.Authentication.ValueBool()
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
			"Missing SMTP Host",
			"The provider cannot create the SMTP client as there is a missing or empty value for the SMTP host. "+
				"Set the host value in the configuration or use the SMTP_HOST environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}
	if port == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("port"),
			"Missing SMTP host port",
			"The provider cannot create the SMTP client as there is a missing or empty value for the SMTP port. "+
				"Set the host value in the configuration or use the SMTP_PORT environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}
	if authentication && username == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("username"),
			"Missing SMTP Username",
			"The provider cannot create the SMTP client as there is a missing or empty value for the SMTP username. "+
				"Set the username value in the configuration or use the SMTP_USERNAME environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if authentication && password == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("password"),
			"Missing SMTP Password",
			"The provider cannot create the SMTP client as there is a missing or empty value for the SMTP password. "+
				"Set the password value in the configuration or use the SMTP_PASSWORD environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	ctx = tflog.SetField(ctx, "smtp_host", host)
	ctx = tflog.SetField(ctx, "smtp_port", port)
	ctx = tflog.SetField(ctx, "smtp_authentication", authentication)
	ctx = tflog.SetField(ctx, "smtp_username", username)
	ctx = tflog.SetField(ctx, "smtp_password", password)
	ctx = tflog.MaskFieldValuesWithFieldKeys(ctx, "smtp_password")

	tflog.Debug(ctx, "Creating SMTP client")

	// Create a new SMTP client using the configuration values
	auth := smtp.Auth(nil)
	if authentication {
		auth = smtp.PlainAuth("", username, password, host)
	}

	client := &client{
		host:     host,
		port:     port,
		username: username,
		auth:     auth,
	}

	// Make the SMTP client available during DataSource and Resource
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
