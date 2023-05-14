package smtp

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource              = &sendMailResource{}
	_ resource.ResourceWithConfigure = &sendMailResource{}
)

// NewOrderResource is a helper function to simplify the provider implementation.
func NewSendMailResource() resource.Resource {
	return &sendMailResource{}
}

// sendMailResource is the resource implementation.
type sendMailResource struct {
	client *client
}

type sendMailModel struct {
	ID         types.String `tfsdk:"id"`
	From       types.String `tfsdk:"from"`
	To         types.List   `tfsdk:"to"`
	Cc         types.List   `tfsdk:"cc"`
	Bcc        types.List   `tfsdk:"bcc"`
	Subject    types.String `tfsdk:"subject"`
	Body       types.String `tfsdk:"body"`
	RenderHtml types.Bool   `tfsdk:"render_html"`
}

// Configure adds the provider configured client to the resource.
func (r *sendMailResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	r.client = req.ProviderData.(*client)
}

// Metadata returns the resource type name.
func (r *sendMailResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_send_mail"
}

// Schema defines the schema for the resource.
func (r *sendMailResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Send a email with smtp. Note: At this moment TLS validation is not support.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description: "Autogenerated id for the resource.",
				Computed:    true,
			},
			"from": schema.StringAttribute{
				Optional:    true,
				Description: "From email address. If not provided, the username used in the smtp auth will be used.",
			},
			"to": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "To email addresses.",
				Required:    true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"cc": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "CC email addresses.",
				Optional:    true,
			},
			"bcc": schema.ListAttribute{
				ElementType: types.StringType,
				Description: "BCC email addresses. ",
				Optional:    true,
			},
			"subject": schema.StringAttribute{
				Required:    true,
				Description: "Subject of the email.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"body": schema.StringAttribute{
				Required:    true,
				Description: "Body of the email.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"render_html": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Boolean flag is identify whether the body is html or plain text. Set this to `true` if body is a HTML content.",
				Default:     booldefault.StaticBool(false),
			},
		},
	}
}

// Create creates the resource and sets the initial Terraform state.
func (r *sendMailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Retrieve values from plan
	var plan sendMailModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	host_port := r.client.host + ":" + r.client.port
	// Connect to the SMTP server using a plain TCP connection.
	conn, err := smtp.Dial(host_port)
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to SMTP server:", err.Error())
		return
	}

	// Upgrade the connection to TLS.
	if r.client.auth != nil {
		err = conn.StartTLS(&tls.Config{ServerName: host_port, InsecureSkipVerify: true})
		if err != nil {
			resp.Diagnostics.AddError("Error upgrading connection to TLS:", err.Error())
			return
		}
	}

	// Authenticate with the SMTP server.
	if r.client.auth != nil {
		err = conn.Auth(r.client.auth)
		if err != nil {
			resp.Diagnostics.AddError("Error authenticating with SMTP server:", err.Error())
			return
		}
	}

	// Set the sender and recipient addresses, and the email message.
	from := plan.From.ValueString()
	if from == "" {
		from = r.client.username
	}

	mime := ""
	if plan.RenderHtml.ValueBool() {
		mime = "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	}

	//to := []string{plan.To.ValueString()}
	receivers := append(plan.To.Elements(), plan.Cc.Elements()...)
	receivers = append(receivers, plan.Bcc.Elements()...)
	receivers = uniqueAttrValue(receivers)
	msg := []byte("To: " + strings.Join(asStringList(plan.To.Elements()), ", ") + "\r\n" +
		"Cc: " + strings.Join(asStringList(plan.Cc.Elements()), ", ") + "\r\n" +
		"Subject: " + plan.Subject.ValueString() + "\r\n" +
		mime +
		"\r\n" +
		plan.Body.ValueString() + "\r\n")

	// Send the email.
	err = conn.Mail(from)
	if err != nil {
		resp.Diagnostics.AddError("Error setting sender address:", err.Error())
		return
	}
	for _, addr := range receivers {
		err = conn.Rcpt(addr.String())
		if err != nil {
			resp.Diagnostics.AddError("Error setting recipient address:", err.Error())
			return
		}
	}
	w, err := conn.Data()
	if err != nil {
		resp.Diagnostics.AddError("Error setting email message:", err.Error())
		return
	}
	_, err = w.Write(msg)
	if err != nil {
		resp.Diagnostics.AddError("Error setting email message:", err.Error())
		return
	}
	err = w.Close()
	if err != nil {
		resp.Diagnostics.AddError("Error sending email:", err.Error())
		return
	}

	tflog.Info(ctx, "Email sent successfully!")
	plan.ID = types.StringValue(fmt.Sprintf("%x", md5.Sum(msg)))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
}

// Read refreshes the Terraform state with the latest data.
func (r *sendMailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
}

// Update updates the resource and sets the updated Terraform state on success.
func (r *sendMailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

	// Retrieve values from plan
	var plan sendMailModel
	diags := req.Plan.Get(ctx, &plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	host_port := r.client.host + ":" + r.client.port
	// Connect to the SMTP server using a plain TCP connection.
	conn, err := smtp.Dial(host_port)
	if err != nil {
		resp.Diagnostics.AddError("Error connecting to SMTP server:", err.Error())
		return
	}

	// Upgrade the connection to TLS.
	err = conn.StartTLS(&tls.Config{ServerName: host_port, InsecureSkipVerify: true})
	if err != nil {
		resp.Diagnostics.AddError("Error upgrading connection to TLS:", err.Error())
		return
	}

	// Authenticate with the SMTP server.
	err = conn.Auth(r.client.auth)
	if err != nil {
		resp.Diagnostics.AddError("Error authenticating with SMTP server:", err.Error())
		return
	}

	// Set the sender and recipient addresses, and the email message.
	// Set the sender and recipient addresses, and the email message.
	from := plan.From.ValueString()
	if from == "" {
		from = r.client.username
	}

	mime := ""
	if plan.RenderHtml.ValueBool() {
		mime = "MIME-version: 1.0;\nContent-Type: text/html; charset=\"UTF-8\";\n\n"
	}

	//to := []string{plan.To.ValueString()}
	receivers := append(plan.To.Elements(), plan.Cc.Elements()...)
	receivers = append(receivers, plan.Bcc.Elements()...)
	receivers = uniqueAttrValue(receivers)
	msg := []byte("To: " + strings.Join(asStringList(plan.To.Elements()), ", ") + "\r\n" +
		"Cc: " + strings.Join(asStringList(plan.Cc.Elements()), ", ") + "\r\n" +
		"Subject: " + plan.Subject.ValueString() + "\r\n" +
		mime +
		"\r\n" +
		plan.Body.ValueString() + "\r\n")

	// Send the email.
	err = conn.Mail(from)
	if err != nil {
		resp.Diagnostics.AddError("Error setting sender address:", err.Error())
		return
	}
	for _, addr := range receivers {
		err = conn.Rcpt(addr.String())
		if err != nil {
			resp.Diagnostics.AddError("Error setting recipient address:", err.Error())
			return
		}
	}
	w, err := conn.Data()
	if err != nil {
		resp.Diagnostics.AddError("Error setting email message:", err.Error())
		return
	}
	_, err = w.Write(msg)
	if err != nil {
		resp.Diagnostics.AddError("Error setting email message:", err.Error())
		return
	}
	err = w.Close()
	if err != nil {
		resp.Diagnostics.AddError("Error sending email:", err.Error())
		return
	}

	tflog.Info(ctx, "Email sent successfully!")
	plan.ID = types.StringValue(fmt.Sprintf("%x", md5.Sum(msg)))

	// Set state to fully populated data
	diags = resp.State.Set(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

}

// Delete deletes the resource and removes the Terraform state on success.
func (r *sendMailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
}

func uniqueAttrValue(arr []attr.Value) []attr.Value {
	occurred := map[attr.Value]bool{}
	result := []attr.Value{}
	for e := range arr {

		// check if already the mapped
		// variable is set to true or not
		if !occurred[arr[e]] {
			occurred[arr[e]] = true

			// Append to result slice.
			result = append(result, arr[e])
		}
	}
	return result
}

// Convert the array of attr.Value to  array of string.
func asStringList(arr []attr.Value) []string {
	result := []string{}
	for _, i := range arr {
		result = append(result, i.String())
	}
	return result
}
