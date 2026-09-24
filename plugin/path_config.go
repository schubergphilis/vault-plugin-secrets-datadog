package plugin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/hashicorp/vault/sdk/framework"
	"github.com/hashicorp/vault/sdk/logical"
)

const (
	pathConfigDef             = "config"
	configStoragePath         = "config"
	pathConfigHelpSynopsis    = "Configure the datadog backend"
	pathConfigHelpDescription = `
	The Datadog secret backend requires credentials for managing
	API and App keys.

	You must provide an API and App key scoped at 
	least with the ability to create an API and 
	App key before using this secrets backend.

	Set site to the Datadog site of the organization, for
	example datadoghq.com (US) or datadoghq.eu (EU). It
	defaults to datadoghq.com.
	`
	defaultSite = "datadoghq.com"
)

// supportedSites returns the Datadog sites known to the API client.
func supportedSites() []string {
	sites := []string{}
	for _, site := range datadog.NewConfiguration().Servers[0].Variables["site"].EnumValues {
		if !contains(sites, site) {
			sites = append(sites, site)
		}
	}
	return sites
}

// siteOrDefault returns the configured site, falling back to the default for
// configurations stored before the site setting existed.
func (c *datadogConfig) siteOrDefault() string {
	if c.Site == "" {
		return defaultSite
	}
	return c.Site
}

type datadogConfig struct {
	APIKey   string `json:"api_key"`
	APIKeyID string `json:"api_key_id"`
	AppKey   string `json:"app_key"`
	AppKeyID string `json:"app_key_id"`
	Site     string `json:"site"`
}

func pathConfig(b *datadogBackend) *framework.Path {

	return &framework.Path{
		Pattern: pathConfigDef,
		Fields: map[string]*framework.FieldSchema{
			"api_key": {
				Type:        framework.TypeString,
				Description: "The API Key for accessing datadog's API",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "API Key",
					Sensitive: true,
				},
			},
			"api_key_id": {
				Type:        framework.TypeString,
				Description: "The ID of the datadog API Key",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "API Key ID",
					Sensitive: false,
				},
			},
			"app_key": {
				Type:        framework.TypeString,
				Description: "The Application Key scoped to admin level priveleges",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Application Key",
					Sensitive: true,
				},
			},
			"app_key_id": {
				Type:        framework.TypeString,
				Description: "The ID of the datadog Application Key",
				Required:    true,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Application Key ID",
					Sensitive: false,
				},
			},
			"site": {
				Type:        framework.TypeString,
				Description: fmt.Sprintf("The Datadog site the organization lives on, e.g. datadoghq.com (US1) or datadoghq.eu (EU1). One of: %s", strings.Join(supportedSites(), ", ")),
				Default:     defaultSite,
				DisplayAttrs: &framework.DisplayAttributes{
					Name:      "Site",
					Sensitive: false,
				},
			},
		},
		Operations: map[logical.Operation]framework.OperationHandler{
			logical.CreateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.ReadOperation: &framework.PathOperation{
				Callback: b.pathConfigRead,
			},
			logical.UpdateOperation: &framework.PathOperation{
				Callback: b.pathConfigWrite,
			},
			logical.DeleteOperation: &framework.PathOperation{
				Callback: b.pathConfigDelete,
			},
		},
		ExistenceCheck:  b.PathConfigExistenceCheck,
		HelpSynopsis:    pathConfigHelpSynopsis,
		HelpDescription: pathConfigHelpDescription,
	}
}

func (b *datadogBackend) pathConfigRead(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {

	config, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	return &logical.Response{
		Data: map[string]interface{}{
			"api_key_id": config.APIKeyID,
			"app_key_id": config.AppKeyID,
			"site":       config.siteOrDefault(),
		},
	}, nil
}

func (b *datadogBackend) pathConfigWrite(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {

	config, err := getConfig(ctx, req.Storage)
	if err != nil {
		return nil, err
	}

	createOperation := (req.Operation == logical.CreateOperation)

	if config == nil {
		if !createOperation {
			return nil, errors.New("config not found during update operation")
		}
		config = new(datadogConfig)
	}

	if apiKey, ok := data.GetOk("api_key"); ok {
		config.APIKey = apiKey.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing API Key in configuration")
	}

	if apiKeyID, ok := data.GetOk("api_key_id"); ok {
		config.APIKeyID = apiKeyID.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing API Key ID in configuration")
	}

	if appKey, ok := data.GetOk("app_key"); ok {
		config.AppKey = appKey.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing Application Key in configuration")
	}

	if appKeyID, ok := data.GetOk("app_key_id"); ok {
		config.AppKeyID = appKeyID.(string)
	} else if !ok && createOperation {
		return nil, fmt.Errorf("missing Application Key ID in configuration")
	}

	if site, ok := data.GetOk("site"); ok {
		config.Site = site.(string)
	} else if createOperation {
		config.Site = defaultSite
	}

	if !contains(supportedSites(), config.siteOrDefault()) {
		return logical.ErrorResponse("unsupported site %q, must be one of: %s", config.Site, strings.Join(supportedSites(), ", ")), nil
	}

	entry, err := logical.StorageEntryJSON(configStoragePath, config)
	if err != nil {
		return nil, err
	}

	if err := req.Storage.Put(ctx, entry); err != nil {
		return nil, err
	}

	b.reset()

	return nil, nil
}

func (b *datadogBackend) pathConfigDelete(ctx context.Context, req *logical.Request, data *framework.FieldData) (*logical.Response, error) {

	err := req.Storage.Delete(ctx, configStoragePath)

	if err == nil {
		b.reset()
	}

	return nil, err
}

func (b *datadogBackend) PathConfigExistenceCheck(ctx context.Context, req *logical.Request, data *framework.FieldData) (bool, error) {

	out, err := req.Storage.Get(ctx, configStoragePath)
	if err != nil {
		return false, fmt.Errorf("existence check failed: %w", err)
	}
	return out != nil, nil
}

func getConfig(ctx context.Context, s logical.Storage) (*datadogConfig, error) {
	entry, err := s.Get(ctx, configStoragePath)
	if err != nil {
		return nil, err
	}

	if entry == nil {
		return nil, nil
	}

	config := new(datadogConfig)
	if err := entry.DecodeJSON(&config); err != nil {
		return nil, fmt.Errorf("error reading root configuration: %w", err)
	}

	return config, nil
}
