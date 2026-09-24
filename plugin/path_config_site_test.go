package plugin

import (
	"context"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func baseConfigData() map[string]interface{} {
	return map[string]interface{}{
		"api_key":    APIKey,
		"api_key_id": APIKeyID,
		"app_key":    AppKey,
		"app_key_id": AppKeyID,
	}
}

func readConfigSite(t *testing.T, b logical.Backend, s logical.Storage) string {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      pathConfigDef,
		Storage:   s,
	})
	require.NoError(t, err)
	return resp.Data["site"].(string)
}

func TestConfigSiteDefaultsToUS(t *testing.T) {
	b, s := getTestBackend(t)
	require.NoError(t, testConfigCreate(t, b, s, baseConfigData()))
	assert.Equal(t, "datadoghq.com", readConfigSite(t, b, s))
}

func TestConfigSiteEU(t *testing.T) {
	b, s := getTestBackend(t)
	d := baseConfigData()
	d["site"] = "datadoghq.eu"
	require.NoError(t, testConfigCreate(t, b, s, d))
	assert.Equal(t, "datadoghq.eu", readConfigSite(t, b, s))

	// updating other fields keeps the configured site
	require.NoError(t, testConfigUpdate(t, b, s, map[string]interface{}{"api_key": APIKey}))
	assert.Equal(t, "datadoghq.eu", readConfigSite(t, b, s))
}

func TestConfigSiteRejectsUnknown(t *testing.T) {
	b, s := getTestBackend(t)
	d := baseConfigData()
	d["site"] = "example.com"
	assert.ErrorContains(t, testConfigCreate(t, b, s, d), "unsupported site")
}

func TestClientUsesConfiguredSite(t *testing.T) {
	for site, want := range map[string]string{
		"":             "https://api.datadoghq.com",
		"datadoghq.eu": "https://api.datadoghq.eu",
	} {
		c, err := NewClient(&datadogConfig{APIKey: APIKey, AppKey: AppKey, Site: site})
		require.NoError(t, err)
		got, err := c.GetConfig().ServerURLWithContext(c.withSite(context.Background()), "v2.KeyManagementApi.CreateAPIKey")
		require.NoError(t, err)
		assert.Equal(t, want, got)
	}
}

func TestDefaultSiteIsSupported(t *testing.T) {
	assert.Contains(t, supportedSites(), defaultSite)
	assert.Contains(t, supportedSites(), "datadoghq.eu")
}
