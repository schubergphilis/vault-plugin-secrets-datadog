package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/DataDog/datadog-api-client-go/v2/api/datadogV2"
)

type datadogClient struct {
	*datadog.APIClient
	site string
}

func NewClient(config *datadogConfig) (*datadogClient, error) {

	if config == nil {
		return nil, errors.New("client configuration was nil")
	}

	// create datadog APIClient
	conf := datadog.NewConfiguration()
	if config.APIKey == "" {
		return nil, errors.New("datadog API key was not provided")
	}
	conf.AddDefaultHeader("DD-API-KEY", config.APIKey)
	if config.AppKey == "" {
		return nil, errors.New("datadog aaplication key was not provided")
	}
	conf.AddDefaultHeader("DD-APPLICATION-KEY", config.AppKey)
	c := datadog.NewAPIClient(conf)

	return &datadogClient{APIClient: c, site: config.siteOrDefault()}, nil
}

// withSite points API calls made with ctx at the configured Datadog site.
func (c *datadogClient) withSite(ctx context.Context) context.Context {
	return context.WithValue(ctx, datadog.ContextServerVariables, map[string]string{"site": c.site})
}

func (c *datadogClient) createAPIKey(ctx context.Context, apiKeyName string) (*datadogAPIKey, error) {

	body := datadogV2.APIKeyCreateRequest{
		Data: datadogV2.APIKeyCreateData{
			Attributes: *datadogV2.NewAPIKeyCreateAttributes(apiKeyName),
			Type:       datadogV2.APIKEYSTYPE_API_KEYS,
		},
	}

	api := datadogV2.NewKeyManagementApi(c.APIClient)

	ddresp, _, err := api.CreateAPIKey(c.withSite(ctx), body)
	if err != nil {
		return nil, fmt.Errorf("error creating datadog API key; %w", err)
	}
	respData := ddresp.GetData()

	return &datadogAPIKey{
		APIKeyID: *respData.Id,
		APIKey:   *respData.Attributes.Key,
	}, err
}

func (c *datadogClient) deleteAPIKey(ctx context.Context, apiKeyID string) error {

	api := datadogV2.NewKeyManagementApi(c.APIClient)

	_, err := api.DeleteAPIKey(c.withSite(ctx), apiKeyID)
	if err != nil {
		return fmt.Errorf("error deleting datadog API key: %w", err)
	}
	return nil
}

func (c *datadogClient) createAppKey(ctx context.Context, name string, scopes []string) (*datadogAppKey, error) {

	// as of v2.14 of the DD API, a call to datadogv2.ApplicationKeyCreateRequest
	// requires the Scopes attributes to be a datadog.NullableList[string]
	ns := datadog.NewNullableList[string](&scopes)

	body := datadogV2.ApplicationKeyCreateRequest{
		Data: datadogV2.ApplicationKeyCreateData{
			Attributes: datadogV2.ApplicationKeyCreateAttributes{
				Name:   name,
				Scopes: *ns,
			},
			Type: datadogV2.APPLICATIONKEYSTYPE_APPLICATION_KEYS,
		},
	}

	api := datadogV2.NewKeyManagementApi(c.APIClient)

	ddresp, _, err := api.CreateCurrentUserApplicationKey(c.withSite(ctx), body)
	if err != nil {
		return nil, fmt.Errorf("error creating datadog application key: %w", err)
	}

	respData := ddresp.GetData()

	return &datadogAppKey{
		AppKeyID: *respData.Id,
		AppKey:   *respData.Attributes.Key,
	}, nil
}

func (c *datadogClient) deleteAppKey(ctx context.Context, appKeyID string) error {

	api := datadogV2.NewKeyManagementApi(c.APIClient)

	_, err := api.DeleteApplicationKey(c.withSite(ctx), appKeyID)
	if err != nil {
		return fmt.Errorf("error deleting datadog application key: %w", err)
	}

	return nil
}

func (c *datadogClient) createPersonalAccessToken(ctx context.Context, name string, scopes []string, expiresAt time.Time) (*datadogAccessToken, error) {

	body := datadogV2.PersonalAccessTokenCreateRequest{
		Data: datadogV2.PersonalAccessTokenCreateData{
			Attributes: datadogV2.PersonalAccessTokenCreateAttributes{
				Name:      name,
				Scopes:    scopes,
				ExpiresAt: expiresAt,
			},
			Type: datadogV2.PERSONALACCESSTOKENSTYPE_PERSONAL_ACCESS_TOKENS,
		},
	}

	api := datadogV2.NewKeyManagementApi(c.APIClient)

	ddresp, _, err := api.CreatePersonalAccessToken(c.withSite(ctx), body)
	if err != nil {
		return nil, fmt.Errorf("error creating datadog personal access token: %w", err)
	}

	data := ddresp.GetData()
	attributes := data.GetAttributes()

	return newAccessToken(data.GetId(), attributes.GetKey())
}

func (c *datadogClient) revokePersonalAccessToken(ctx context.Context, tokenID string) error {

	api := datadogV2.NewKeyManagementApi(c.APIClient)

	httpResp, err := api.RevokePersonalAccessToken(c.withSite(ctx), tokenID)
	if err != nil && !isNotFound(httpResp) {
		return fmt.Errorf("error revoking datadog personal access token: %w", err)
	}
	return nil
}

func (c *datadogClient) createServiceAccessToken(ctx context.Context, serviceAccountID, name string, scopes []string, expiresAt time.Time) (*datadogAccessToken, error) {

	body := datadogV2.ServiceAccountAccessTokenCreateRequest{
		Data: datadogV2.ServiceAccountAccessTokenCreateData{
			Attributes: datadogV2.ServiceAccountAccessTokenCreateAttributes{
				Name:      name,
				Scopes:    scopes,
				ExpiresAt: &expiresAt,
			},
			Type: datadogV2.SERVICEACCESSTOKENSTYPE_SERVICE_ACCESS_TOKENS,
		},
	}

	api := datadogV2.NewServiceAccountsApi(c.APIClient)

	ddresp, _, err := api.CreateServiceAccountAccessToken(c.withSite(ctx), serviceAccountID, body)
	if err != nil {
		return nil, fmt.Errorf("error creating datadog service account access token: %w", err)
	}

	data := ddresp.GetData()
	attributes := data.GetAttributes()

	return newAccessToken(data.GetId(), attributes.GetKey())
}

func (c *datadogClient) revokeServiceAccessToken(ctx context.Context, serviceAccountID, tokenID string) error {

	api := datadogV2.NewServiceAccountsApi(c.APIClient)

	httpResp, err := api.RevokeServiceAccountAccessToken(c.withSite(ctx), serviceAccountID, tokenID)
	if err != nil && !isNotFound(httpResp) {
		return fmt.Errorf("error revoking datadog service account access token: %w", err)
	}
	return nil
}

// isNotFound reports whether datadog answered 404, i.e. the token is already gone.
func isNotFound(resp *http.Response) bool {
	return resp != nil && resp.StatusCode == http.StatusNotFound
}

// newAccessToken guards against a create response without the fields Vault
// needs to hand out and later revoke the token.
func newAccessToken(id, key string) (*datadogAccessToken, error) {
	if id == "" || key == "" {
		return nil, errors.New("datadog returned an access token without an ID or key")
	}
	return &datadogAccessToken{ID: id, Token: key}, nil
}
