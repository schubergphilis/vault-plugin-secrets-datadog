package plugin

import (
	"context"
	"strconv"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/require"
)

const (
	roleName   = "testdatadog"
	testTTL    = int64(120)
	testMaxTTL = int64(3600)
)

var (
	scopes = []string{"incident_read", "usage_read"}
)

// TestUserRole uses a mock backend to check
// role create, read, update, and delete.
func TestDatadogRole(t *testing.T) {
	b, s := getTestBackend(t)

	t.Run("List All Roles", func(t *testing.T) {
		for i := 1; i <= 10; i++ {
			_, err := testTokenRoleCreate(t, b, s,
				roleName+strconv.Itoa(i),
				map[string]interface{}{
					"app_key_scopes": scopes,
					"ttl":            testTTL,
					"max_ttl":        testMaxTTL,
				})
			require.NoError(t, err)
		}

		resp, err := testTokenRoleList(t, b, s)
		require.NoError(t, err)
		require.Len(t, resp.Data["keys"].([]string), 10)
	})

	t.Run("Create Datadog Role - pass", func(t *testing.T) {
		resp, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
			"app_key_scopes": scopes,
			"ttl":            testTTL,
			"max_ttl":        testMaxTTL,
		})

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Read Datadog Role", func(t *testing.T) {
		resp, err := testTokenRoleRead(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.NotNil(t, resp)
		require.Equal(t, resp.Data["app_key_scopes"], scopes)
	})
	t.Run("Update Datadog Role", func(t *testing.T) {
		resp, err := testTokenRoleUpdate(t, b, s, map[string]interface{}{
			"ttl":     "1m",
			"max_ttl": "5h",
		})

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.Nil(t, resp)
	})

	t.Run("Re-read Datadog Role", func(t *testing.T) {
		resp, err := testTokenRoleRead(t, b, s)

		require.Nil(t, err)
		require.Nil(t, resp.Error())
		require.NotNil(t, resp)
		require.Equal(t, resp.Data["app_key_scopes"], scopes)
	})

	t.Run("Delete Datadog Role", func(t *testing.T) {
		_, err := testTokenRoleDelete(t, b, s)

		require.NoError(t, err)
	})
}

// Utility function to create a role while, returning any response (including errors)
func testTokenRoleCreate(t *testing.T, b *datadogBackend, s logical.Storage, name string, d map[string]interface{}) (*logical.Response, error) {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.CreateOperation,
		Path:      "roles/" + name,
		Data:      d,
		Storage:   s,
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// Utility function to update a role while, returning any response (including errors)
func testTokenRoleUpdate(t *testing.T, b *datadogBackend, s logical.Storage, d map[string]interface{}) (*logical.Response, error) {
	t.Helper()
	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      "roles/" + roleName,
		Data:      d,
		Storage:   s,
	})

	if err != nil {
		return nil, err
	}

	if resp != nil && resp.IsError() {
		t.Fatal(resp.Error())
	}
	return resp, nil
}

// Utility function to read a role and return any errors
func testTokenRoleRead(t *testing.T, b *datadogBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()
	return b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      "roles/" + roleName,
		Storage:   s,
	})
}

// Utility function to list roles and return any errors
func testTokenRoleList(t *testing.T, b *datadogBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()
	return b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ListOperation,
		Path:      "roles/",
		Storage:   s,
	})
}

// Utility function to delete a role and return any errors
func testTokenRoleDelete(t *testing.T, b *datadogBackend, s logical.Storage) (*logical.Response, error) {
	t.Helper()
	return b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.DeleteOperation,
		Path:      "roles/" + roleName,
		Storage:   s,
	})
}

func TestRolePartialUpdateKeepsOtherFields(t *testing.T) {
	b, s := getTestBackend(t)
	_, err := testTokenRoleCreate(t, b, s, roleName, map[string]interface{}{
		"app_key_scopes":      "usage_read",
		"access_token_scopes": "metrics_read, ,dashboards_read",
		"ttl":                 testTTL,
		"max_ttl":             testMaxTTL,
	})
	require.NoError(t, err)

	// Vault's router picks create vs update from the existence check
	req := &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      pathRoleDef + roleName,
		Storage:   s,
		Data:      map[string]interface{}{"service_account_id": "sa-123"},
	}
	checkFound, exists, err := b.HandleExistenceCheck(context.Background(), req)
	require.NoError(t, err)
	require.True(t, checkFound)
	require.True(t, exists, "existing role must be reported as existing")

	resp, err := b.HandleRequest(context.Background(), req)
	require.NoError(t, err)
	require.Nil(t, resp)

	resp, err = testTokenRoleRead(t, b, s)
	require.NoError(t, err)
	require.Equal(t, []string{"usage_read"}, resp.Data["app_key_scopes"])
	require.Equal(t, []string{"metrics_read", "dashboards_read"}, resp.Data["access_token_scopes"])
	require.Equal(t, "sa-123", resp.Data["service_account_id"])
	require.Equal(t, float64(testTTL), resp.Data["ttl"])
	require.Equal(t, float64(testMaxTTL), resp.Data["max_ttl"])
}
