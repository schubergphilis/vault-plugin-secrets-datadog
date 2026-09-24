package plugin

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DataDog/datadog-api-client-go/v2/api/datadog"
	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeDatadog records the requests it receives and answers token creation
// with a fixed token.
type fakeDatadog struct {
	mu           sync.Mutex
	requests     []fakeRequest
	revokeStatus int
}

type fakeRequest struct {
	Method string
	Path   string
	Body   map[string]interface{}
}

func (f *fakeDatadog) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests = append(f.requests, fakeRequest{Method: r.Method, Path: r.URL.Path, Body: decodeBody(r)})
	f.mu.Unlock()

	if r.Method == http.MethodDelete {
		status := http.StatusNoContent
		if f.revokeStatus != 0 {
			status = f.revokeStatus
		}
		w.WriteHeader(status)
		return
	}
	tokenType := "personal_access_tokens"
	if strings.HasPrefix(r.URL.Path, "/api/v2/service_accounts/") {
		tokenType = "service_access_tokens"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_, _ = io.WriteString(w, `{"data":{"id":"token-id-1","type":"`+tokenType+`","attributes":{"key":"ddpat_or_ddsat_secret"}}}`)
}

func (f *fakeDatadog) last() fakeRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.requests[len(f.requests)-1]
}

func decodeBody(r *http.Request) map[string]interface{} {
	reader := io.Reader(r.Body)
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil
		}
		reader = gz
	}
	var body map[string]interface{}
	_ = json.NewDecoder(reader).Decode(&body)
	return body
}

// getFakeDatadogBackend returns a configured backend whose client talks to a fake datadog API.
func getFakeDatadogBackend(t *testing.T) (*datadogBackend, logical.Storage, *fakeDatadog) {
	t.Helper()
	b, s := getTestBackend(t)
	require.NoError(t, testConfigCreate(t, b, s, baseConfigData()))

	fake := &fakeDatadog{}
	srv := httptest.NewServer(fake)
	t.Cleanup(srv.Close)

	client, err := NewClient(&datadogConfig{APIKey: APIKey, AppKey: AppKey})
	require.NoError(t, err)
	client.GetConfig().Servers = datadog.ServerConfigurations{{URL: srv.URL}}
	b.client = client

	return b, s, fake
}

func writeTokenRole(t *testing.T, b *datadogBackend, s logical.Storage, data map[string]interface{}) {
	t.Helper()
	resp, err := testTokenRoleCreate(t, b, s, roleName, data)
	require.NoError(t, err)
	require.Nil(t, resp)
}

func readCredential(t *testing.T, b logical.Backend, s logical.Storage, path string) *logical.Response {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      path,
		Storage:   s,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	return resp
}

func revokeCredential(t *testing.T, b logical.Backend, s logical.Storage, secret *logical.Secret) {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.RevokeOperation,
		Secret:    secret,
		Storage:   s,
	})
	require.NoError(t, err)
	require.False(t, resp != nil && resp.IsError(), "revoke returned %v", resp)
}

func TestPersonalAccessToken(t *testing.T) {
	b, s, fake := getFakeDatadogBackend(t)
	writeTokenRole(t, b, s, map[string]interface{}{
		"access_token_scopes": "dashboards_read,monitors_read",
		"ttl":                 testTTL,
		"max_ttl":             testMaxTTL,
	})

	resp := readCredential(t, b, s, personalAccessTokenPath+roleName)
	require.False(t, resp.IsError(), "%v", resp)
	assert.Equal(t, "ddpat_or_ddsat_secret", resp.Data["token"])
	assert.Equal(t, time.Duration(testTTL)*time.Second, resp.Secret.TTL)

	created := fake.last()
	assert.Equal(t, http.MethodPost, created.Method)
	assert.Equal(t, "/api/v2/personal_access_tokens", created.Path)
	attributes := created.Body["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	assert.Equal(t, []interface{}{"dashboards_read", "monitors_read"}, attributes["scopes"])
	assertExpiresAtLeastADay(t, attributes["expires_at"])

	revokeCredential(t, b, s, resp.Secret)
	revoked := fake.last()
	assert.Equal(t, http.MethodDelete, revoked.Method)
	assert.Equal(t, "/api/v2/personal_access_tokens/token-id-1", revoked.Path)
}

func TestServiceAccessToken(t *testing.T) {
	b, s, fake := getFakeDatadogBackend(t)
	writeTokenRole(t, b, s, map[string]interface{}{
		"access_token_scopes": "metrics_read",
		"service_account_id":  "sa-123",
	})

	resp := readCredential(t, b, s, serviceAccessTokenPath+roleName)
	require.False(t, resp.IsError(), "%v", resp)
	assert.Equal(t, "ddpat_or_ddsat_secret", resp.Data["token"])

	created := fake.last()
	assert.Equal(t, "/api/v2/service_accounts/sa-123/access_tokens", created.Path)
	attributes := created.Body["data"].(map[string]interface{})["attributes"].(map[string]interface{})
	assertExpiresAtLeastADay(t, attributes["expires_at"])

	revokeCredential(t, b, s, resp.Secret)
	revoked := fake.last()
	assert.Equal(t, http.MethodDelete, revoked.Method)
	assert.Equal(t, "/api/v2/service_accounts/sa-123/access_tokens/token-id-1", revoked.Path)
}

func TestAccessTokenRoleRequirements(t *testing.T) {
	b, s, fake := getFakeDatadogBackend(t)

	resp := readCredential(t, b, s, personalAccessTokenPath+"missing")
	assert.ErrorContains(t, resp.Error(), "not found")

	writeTokenRole(t, b, s, map[string]interface{}{"app_key_scopes": "usage_read"})
	resp = readCredential(t, b, s, personalAccessTokenPath+roleName)
	assert.ErrorContains(t, resp.Error(), "no access_token_scopes")

	writeTokenRole(t, b, s, map[string]interface{}{"access_token_scopes": "metrics_read"})
	resp = readCredential(t, b, s, serviceAccessTokenPath+roleName)
	assert.ErrorContains(t, resp.Error(), "no service_account_id")

	assert.Empty(t, fake.requests, "no datadog calls expected for invalid roles")
}

func TestAccessTokenExpiry(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string]struct {
		roleMax, systemMax, want time.Duration
	}{
		"role max_ttl above minimum":  {72 * time.Hour, 768 * time.Hour, 72 * time.Hour},
		"role max_ttl below minimum":  {time.Hour, 768 * time.Hour, minAccessTokenLifetime},
		"no role max_ttl uses system": {0, 768 * time.Hour, 768 * time.Hour},
		"role max_ttl capped by sys":  {1000 * time.Hour, 768 * time.Hour, 768 * time.Hour},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, now.Add(tc.want+accessTokenExpiryMargin), accessTokenExpiry(now, tc.roleMax, tc.systemMax))
		})
	}
}

func assertExpiresAtLeastADay(t *testing.T, raw interface{}) {
	t.Helper()
	expiresAt, err := time.Parse(time.RFC3339, raw.(string))
	require.NoError(t, err)
	assert.True(t, time.Until(expiresAt) > 24*time.Hour, "expires_at %s must be over 24h away", expiresAt)
}

func TestAccessTokenRevokeTreatsNotFoundAsRevoked(t *testing.T) {
	b, s, fake := getFakeDatadogBackend(t)
	writeTokenRole(t, b, s, map[string]interface{}{"access_token_scopes": "metrics_read"})
	resp := readCredential(t, b, s, personalAccessTokenPath+roleName)

	fake.revokeStatus = http.StatusNotFound
	revokeCredential(t, b, s, resp.Secret)

	fake.revokeStatus = http.StatusForbidden
	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.RevokeOperation,
		Secret:    resp.Secret,
		Storage:   s,
	})
	assert.Error(t, err, "other revoke failures must still surface")
}

func TestAccessTokenRenewCappedAtTokenExpiry(t *testing.T) {
	b, s, _ := getFakeDatadogBackend(t)
	writeTokenRole(t, b, s, map[string]interface{}{
		"access_token_scopes": "metrics_read",
		"max_ttl":             int64(48 * 3600),
	})
	resp := readCredential(t, b, s, personalAccessTokenPath+roleName)
	secret := resp.Secret
	secret.IssueTime = time.Now()

	// raising the role max_ttl must not let the lease outlive the token
	writeTokenRole(t, b, s, map[string]interface{}{"max_ttl": int64(720 * 3600)})
	renewed, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.RenewOperation,
		Secret:    secret,
		Storage:   s,
	})
	require.NoError(t, err)
	assert.InDelta(t, (48 * time.Hour).Seconds(), renewed.Secret.MaxTTL.Seconds(), 5)
}
