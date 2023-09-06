package provider

import (
	_ "bytes"
	"context"
	_ "fmt"
	"strings"

	_ "github.com/fbreckle/go-netbox/netbox/client/status"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	_ "golang.org/x/exp/slices"
	"os"
)

// netboxProviderModel maps provider schema data to a Go type.
type netboxProviderModel struct {
	ServerURL                   types.String `tfsdk:"server_url"`
	ApiToken                    types.String `tfsdk:"api_token"`
	StripTrailingSlashesFromURL types.Bool   `tfsdk:"strip_trailing_slashes_from_url"`
}

// Schema defines the provider-level schema for configuration data.
func (p *netboxProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"server_url": schema.StringAttribute{
				Required: true,
			},
			"api_token": schema.StringAttribute{
				Optional: true,
			},
			"strip_trailing_slashes_from_url": schema.BoolAttribute{
				Optional: true,
			},
		},
	}
}

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &netboxProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &netboxProvider{
			version: version,
		}
	}
}

// netboxProvider is the provider implementation.
type netboxProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// Metadata returns the provider type name.
func (p *netboxProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "netbox"
	resp.Version = p.version
}

// Configure prepares a HashiCups API client for data sources and resources.
func (p *netboxProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config netboxProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if config.ServerURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("server_url"),
			"Unknown NetBox Server URL",
			"The provider cannot create the NetBox API client as there is an unknown configuration value for the NetBox Server URL. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the NETBOX_SERVER_URL environment variable.",
		)
	}

	if config.ApiToken.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Unknown NetBox API Token",
			"The provider cannot create the NetBox API client as there is an unknown configuration value for the NetBox API Token. "+
				"Either target apply the source of the value first, set the value statically in the configuration, or use the NETBOX_API_TOKEN environment variable.",
		)
	}

	if config.StripTrailingSlashesFromURL.IsUnknown() {
		resp.Diagnostics.AddAttributeError(
			path.Root("strip_trailing_slashes_from_url"),
			"wip", "wip",
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	serverURL := os.Getenv("NETBOX_SERVER_URL")
	apiToken := os.Getenv("NETBOX_API_TOKEN")

	if !config.ServerURL.IsNull() {
		serverURL = config.ServerURL.ValueString()
	}

	if serverURL == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("server_url"),
			"Missing NetBox Server URL",
			"The provider cannot create the NetBox API client as there is a missing configuration value for the NetBox Server URL. "+
				"Set the host value in the configuration or use the NETBOX_SERVER_URL environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	if !config.ApiToken.IsNull() {
		apiToken = config.ApiToken.ValueString()
	}

	if apiToken == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("api_token"),
			"Missing NetBox API Token",
			"The provider cannot create the NetBox API client as there is a missing configuration value for the NetBox API Token. "+
				"Set the host value in the configuration or use the NETBOX_API_TOKEN environment variable. "+
				"If either is already set, ensure the value is not empty.",
		)
	}

	stripTrailingSlashesFromURLEnv := os.Getenv("NETBOX_STRIP_TRAILING_SLASHES_FROM_URL")
	var stripTrailingSlashesFromURL bool = true
	if stripTrailingSlashesFromURLEnv == "false" {
		stripTrailingSlashesFromURL = false
	}

	if !config.StripTrailingSlashesFromURL.IsNull() {
		stripTrailingSlashesFromURL = config.StripTrailingSlashesFromURL.ValueBool()
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// End boilerplate part

	// Unless explicitly switched off, strip trailing slashes from the server url
	// Trailing slashes cause errors as seen in https://github.com/e-breuninger/terraform-provider-netbox/issues/198
	// and https://github.com/e-breuninger/terraform-provider-netbox/issues/300

	if stripTrailingSlashesFromURL {
		trimmed := false

		// This is Go's poor man's while loop
		for strings.HasSuffix(serverURL, "/") {
			serverURL = strings.TrimRight(serverURL, "/")
			trimmed = true
		}
		if trimmed {
			resp.Diagnostics.AddAttributeWarning(
				path.Root("strip_trailing_slashes_from_url"),
				"Stripped trailing slashes from the `server_url` parameter",
				"Trailing slashes in the `server_url` parameter lead to problems in most setups, so all trailing slashes were stripped. Use the `strip_trailing_slashes_from_url` parameter to disable this feature or remove all trailing slashes in the `server_url` to disable this warning.",
			)
		}
	}

	// Create a new NetBox client using the configuration values
	netboxConfig := Config{
		APIToken:  apiToken,
		ServerURL: serverURL,
	}
	client, err := netboxConfig.Client()
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create NetBox API Client",
			"An unexpected error occurred when creating the NetBox API client. "+
				"If the error is not clear, please contact the provider developers.\n\n"+
				"NetBox Client Error: "+err.Error(),
		)
		return
	}
	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *netboxProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewClusterTypeDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *netboxProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}

// This makes the description contain the default value, particularly useful for the docs
// From https://github.com/hashicorp/terraform-plugin-docs/issues/65#issuecomment-1152842370
//func init() {
//	schema.DescriptionKind = schema.StringMarkdown
//
//	schema.SchemaDescriptionBuilder = func(s *schema.Schema) string {
//		desc := s.Description
//		desc = strings.TrimSpace(desc)
//
//		if !bytes.HasSuffix([]byte(s.Description), []byte(".")) && s.Description != "" {
//			desc += "."
//		}
//
//		if s.AtLeastOneOf != nil && len(s.AtLeastOneOf) > 0 {
//			atLeastOneOf := make([]string, len(s.AtLeastOneOf))
//			for i, l := range s.AtLeastOneOf {
//				atLeastOneOf[i] = fmt.Sprintf("`%s`", l)
//			}
//			desc += fmt.Sprintf(" At least one of %s must be given.", joinStringWithFinalConjunction(atLeastOneOf, ", ", "or"))
//		}
//
//		if s.ExactlyOneOf != nil && len(s.ExactlyOneOf) > 0 {
//			exactlyOneOf := make([]string, len(s.ExactlyOneOf))
//			for i, l := range s.ExactlyOneOf {
//				exactlyOneOf[i] = fmt.Sprintf("`%s`", l)
//			}
//			desc += fmt.Sprintf(" Exactly one of %s must be given.", joinStringWithFinalConjunction(exactlyOneOf, ", ", "or"))
//		}
//
//		if s.RequiredWith != nil && len(s.RequiredWith) > 0 {
//			requires := make([]string, len(s.RequiredWith))
//			for i, c := range s.RequiredWith {
//				requires[i] = fmt.Sprintf("`%s`", c)
//			}
//			desc += fmt.Sprintf(" Required when %s is set.", joinStringWithFinalConjunction(requires, ", ", "and"))
//		}
//
//		if s.ConflictsWith != nil && len(s.ConflictsWith) > 0 {
//			conflicts := make([]string, len(s.ConflictsWith))
//			for i, c := range s.ConflictsWith {
//				conflicts[i] = fmt.Sprintf("`%s`", c)
//			}
//			desc += fmt.Sprintf(" Conflicts with %s.", joinStringWithFinalConjunction(conflicts, ", ", "and"))
//		}
//
//		if s.Default != nil {
//			if s.Default == "" {
//				desc += " Defaults to `\"\"`."
//			} else {
//				desc += fmt.Sprintf(" Defaults to `%v`.", s.Default)
//			}
//		}
//
//		return strings.TrimSpace(desc)
//	}
//}
//
//// Provider returns a schema.Provider for Netbox.
//func Provider() *schema.Provider {
//	provider := &schema.Provider{
//		ResourcesMap: map[string]*schema.Resource{
//			"netbox_available_ip_address":       resourceNetboxAvailableIPAddress(),
//			"netbox_virtual_machine":            resourceNetboxVirtualMachine(),
//			"netbox_cluster_type":               resourceNetboxClusterType(),
//			"netbox_cluster":                    resourceNetboxCluster(),
//			"netbox_contact":                    resourceNetboxContact(),
//			"netbox_contact_group":              resourceNetboxContactGroup(),
//			"netbox_contact_assignment":         resourceNetboxContactAssignment(),
//			"netbox_contact_role":               resourceNetboxContactRole(),
//			"netbox_device":                     resourceNetboxDevice(),
//			"netbox_device_interface":           resourceNetboxDeviceInterface(),
//			"netbox_device_type":                resourceNetboxDeviceType(),
//			"netbox_manufacturer":               resourceNetboxManufacturer(),
//			"netbox_tenant":                     resourceNetboxTenant(),
//			"netbox_tenant_group":               resourceNetboxTenantGroup(),
//			"netbox_vrf":                        resourceNetboxVrf(),
//			"netbox_ip_address":                 resourceNetboxIPAddress(),
//			"netbox_interface":                  resourceNetboxInterface(),
//			"netbox_service":                    resourceNetboxService(),
//			"netbox_platform":                   resourceNetboxPlatform(),
//			"netbox_prefix":                     resourceNetboxPrefix(),
//			"netbox_available_prefix":           resourceNetboxAvailablePrefix(),
//			"netbox_primary_ip":                 resourceNetboxPrimaryIP(),
//			"netbox_device_primary_ip":          resourceNetboxDevicePrimaryIP(),
//			"netbox_device_role":                resourceNetboxDeviceRole(),
//			"netbox_tag":                        resourceNetboxTag(),
//			"netbox_cluster_group":              resourceNetboxClusterGroup(),
//			"netbox_site":                       resourceNetboxSite(),
//			"netbox_vlan":                       resourceNetboxVlan(),
//			"netbox_vlan_group":                 resourceNetboxVlanGroup(),
//			"netbox_ipam_role":                  resourceNetboxIpamRole(),
//			"netbox_ip_range":                   resourceNetboxIPRange(),
//			"netbox_region":                     resourceNetboxRegion(),
//			"netbox_aggregate":                  resourceNetboxAggregate(),
//			"netbox_rir":                        resourceNetboxRir(),
//			"netbox_route_target":               resourceNetboxRouteTarget(),
//			"netbox_circuit":                    resourceNetboxCircuit(),
//			"netbox_circuit_type":               resourceNetboxCircuitType(),
//			"netbox_circuit_provider":           resourceNetboxCircuitProvider(),
//			"netbox_circuit_termination":        resourceNetboxCircuitTermination(),
//			"netbox_user":                       resourceNetboxUser(),
//			"netbox_permission":                 resourceNetboxPermission(),
//			"netbox_token":                      resourceNetboxToken(),
//			"netbox_custom_field":               resourceCustomField(),
//			"netbox_asn":                        resourceNetboxAsn(),
//			"netbox_location":                   resourceNetboxLocation(),
//			"netbox_site_group":                 resourceNetboxSiteGroup(),
//			"netbox_rack":                       resourceNetboxRack(),
//			"netbox_rack_role":                  resourceNetboxRackRole(),
//			"netbox_rack_reservation":           resourceNetboxRackReservation(),
//			"netbox_cable":                      resourceNetboxCable(),
//			"netbox_device_console_port":        resourceNetboxDeviceConsolePort(),
//			"netbox_device_console_server_port": resourceNetboxDeviceConsoleServerPort(),
//			"netbox_device_power_port":          resourceNetboxDevicePowerPort(),
//			"netbox_device_power_outlet":        resourceNetboxDevicePowerOutlet(),
//			"netbox_device_front_port":          resourceNetboxDeviceFrontPort(),
//			"netbox_device_rear_port":           resourceNetboxDeviceRearPort(),
//			"netbox_device_module_bay":          resourceNetboxDeviceModuleBay(),
//			"netbox_module":                     resourceNetboxModule(),
//			"netbox_module_type":                resourceNetboxModuleType(),
//			"netbox_power_feed":                 resourceNetboxPowerFeed(),
//			"netbox_power_panel":                resourceNetboxPowerPanel(),
//			"netbox_inventory_item_role":        resourceNetboxInventoryItemRole(),
//			"netbox_inventory_item":             resourceNetboxInventoryItem(),
//			"netbox_webhook":                    resourceNetboxWebhook(),
//		},
//		DataSourcesMap: map[string]*schema.Resource{
//			"netbox_asn":              dataSourceNetboxAsn(),
//			"netbox_asns":             dataSourceNetboxAsns(),
//			"netbox_cluster":          dataSourceNetboxCluster(),
//			"netbox_cluster_group":    dataSourceNetboxClusterGroup(),
//			"netbox_cluster_type":     dataSourceNetboxClusterType(),
//			"netbox_contact":          dataSourceNetboxContact(),
//			"netbox_contact_role":     dataSourceNetboxContactRole(),
//			"netbox_contact_group":    dataSourceNetboxContactGroup(),
//			"netbox_tenant":           dataSourceNetboxTenant(),
//			"netbox_tenants":          dataSourceNetboxTenants(),
//			"netbox_tenant_group":     dataSourceNetboxTenantGroup(),
//			"netbox_vrf":              dataSourceNetboxVrf(),
//			"netbox_vrfs":             dataSourceNetboxVrfs(),
//			"netbox_platform":         dataSourceNetboxPlatform(),
//			"netbox_prefix":           dataSourceNetboxPrefix(),
//			"netbox_prefixes":         dataSourceNetboxPrefixes(),
//			"netbox_devices":          dataSourceNetboxDevices(),
//			"netbox_device_role":      dataSourceNetboxDeviceRole(),
//			"netbox_device_type":      dataSourceNetboxDeviceType(),
//			"netbox_site":             dataSourceNetboxSite(),
//			"netbox_tag":              dataSourceNetboxTag(),
//			"netbox_virtual_machines": dataSourceNetboxVirtualMachine(),
//			"netbox_interfaces":       dataSourceNetboxInterfaces(),
//			"netbox_ipam_role":        dataSourceNetboxIPAMRole(),
//			"netbox_route_target":     dataSourceNetboxRouteTarget(),
//			"netbox_ip_addresses":     dataSourceNetboxIPAddresses(),
//			"netbox_ip_range":         dataSourceNetboxIPRange(),
//			"netbox_region":           dataSourceNetboxRegion(),
//			"netbox_vlan":             dataSourceNetboxVlan(),
//			"netbox_vlans":            dataSourceNetboxVlans(),
//			"netbox_vlan_group":       dataSourceNetboxVlanGroup(),
//			"netbox_site_group":       dataSourceNetboxSiteGroup(),
//			"netbox_racks":            dataSourceNetboxRacks(),
//			"netbox_rack_role":        dataSourceNetboxRackRole(),
//		},
//		Schema: map[string]*schema.Schema{
//			"server_url": {
//				Type:        schema.TypeString,
//				Required:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_SERVER_URL", nil),
//				Description: "Location of Netbox server including scheme (http or https) and optional port. Can be set via the `NETBOX_SERVER_URL` environment variable.",
//			},
//			"api_token": {
//				Type:        schema.TypeString,
//				Required:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_API_TOKEN", nil),
//				Description: "Netbox API authentication token. Can be set via the `NETBOX_API_TOKEN` environment variable.",
//			},
//			"allow_insecure_https": {
//				Type:        schema.TypeBool,
//				Optional:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_ALLOW_INSECURE_HTTPS", false),
//				Description: "Flag to set whether to allow https with invalid certificates. Can be set via the `NETBOX_ALLOW_INSECURE_HTTPS` environment variable. Defaults to `false`.",
//			},
//			"headers": {
//				Type:        schema.TypeMap,
//				Optional:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_HEADERS", map[string]interface{}{}),
//				Description: "Set these header on all requests to Netbox. Can be set via the `NETBOX_HEADERS` environment variable.",
//			},
//			"strip_trailing_slashes_from_url": {
//				Type:        schema.TypeBool,
//				Optional:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_STRIP_TRAILING_SLASHES_FROM_URL", true),
//				Description: "If true, strip trailing slashes from the `server_url` parameter and print a warning when doing so. Note that using trailing slashes in the `server_url` parameter will usually lead to errors. Can be set via the `NETBOX_STRIP_TRAILING_SLASHES_FROM_URL` environment variable. Defaults to `true`.",
//			},
//			"skip_version_check": {
//				Type:        schema.TypeBool,
//				Optional:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_SKIP_VERSION_CHECK", false),
//				Description: "If true, do not try to determine the running Netbox version at provider startup. Disables warnings about possibly unsupported Netbox version. Also useful for local testing on terraform plans. Can be set via the `NETBOX_SKIP_VERSION_CHECK` environment variable. Defaults to `false`.",
//			},
//			"request_timeout": {
//				Type:        schema.TypeInt,
//				Optional:    true,
//				DefaultFunc: schema.EnvDefaultFunc("NETBOX_REQUEST_TIMEOUT", 10),
//				Description: "Netbox API HTTP request timeout in seconds. Can be set via the `NETBOX_REQUEST_TIMEOUT` environment variable.",
//			},
//		},
//		ConfigureContextFunc: providerConfigure,
//	}
//	return provider
//}
//
//func providerConfigure(ctx context.Context, data *schema.ResourceData) (interface{}, diag.Diagnostics) {
//	var diags diag.Diagnostics
//
//	config := Config{
//		APIToken:                    data.Get("api_token").(string),
//		AllowInsecureHTTPS:          data.Get("allow_insecure_https").(bool),
//		Headers:                     data.Get("headers").(map[string]interface{}),
//		RequestTimeout:              data.Get("request_timeout").(int),
//		StripTrailingSlashesFromURL: data.Get("strip_trailing_slashes_from_url").(bool),
//	}
//
//	serverURL := data.Get("server_url").(string)
//
//	// Unless explicitly switched off, strip trailing slashes from the server url
//	// Trailing slashes cause errors as seen in https://github.com/e-breuninger/terraform-provider-netbox/issues/198
//	// and https://github.com/e-breuninger/terraform-provider-netbox/issues/300
//	stripTrailingSlashesFromURL := data.Get("strip_trailing_slashes_from_url").(bool)
//
//	if stripTrailingSlashesFromURL {
//		trimmed := false
//
//		// This is Go's poor man's while loop
//		for strings.HasSuffix(serverURL, "/") {
//			serverURL = strings.TrimRight(serverURL, "/")
//			trimmed = true
//		}
//		if trimmed {
//			diags = append(diags, diag.Diagnostic{
//				Severity: diag.Warning,
//				Summary:  "Stripped trailing slashes from the `server_url` parameter",
//				Detail:   "Trailing slashes in the `server_url` parameter lead to problems in most setups, so all trailing slashes were stripped. Use the `strip_trailing_slashes_from_url` parameter to disable this feature or remove all trailing slashes in the `server_url` to disable this warning.",
//			})
//		}
//	}
//
//	config.ServerURL = serverURL
//
//	netboxClient, clientError := config.Client()
//	if clientError != nil {
//		return nil, diag.FromErr(clientError)
//	}
//
//	// Unless explicitly switched off, use the client to retrieve the Netbox version
//	// so we can determine compatibility of the provider with the used Netbox
//	skipVersionCheck := data.Get("skip_version_check").(bool)
//
//	if !skipVersionCheck {
//		req := status.NewStatusListParams()
//		res, err := netboxClient.Status.StatusList(req, nil)
//
//		if err != nil {
//			return nil, diag.FromErr(err)
//		}
//
//		netboxVersion := res.GetPayload().(map[string]interface{})["netbox-version"].(string)
//
//		supportedVersions := []string{"3.5.1", "3.5.2", "3.5.3", "3.5.4", "3.5.6", "3.5.7", "3.5.8", "3.5.9"}
//
//		if !slices.Contains(supportedVersions, netboxVersion) {
//			// Currently, there is no way to test these warnings. There is an issue to track this: https://github.com/hashicorp/terraform-plugin-sdk/issues/864
//			diags = append(diags, diag.Diagnostic{
//				Severity: diag.Warning,
//				Summary:  "Possibly unsupported Netbox version",
//				Detail:   fmt.Sprintf("Your Netbox version is v%v. The provider was successfully tested against the following versions:\n\n  %v\n\nUnexpected errors may occur.", netboxVersion, strings.Join(supportedVersions, ", ")),
//			})
//		}
//	}
//
//	return netboxClient, diags
//}