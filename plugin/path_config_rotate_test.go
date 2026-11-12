package plugin

import (
	"context"
	"testing"

	"github.com/hashicorp/vault/sdk/logical"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigRotateReadIsUnsupported(t *testing.T) {
	b, s := getTestBackend(t)

	_, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.ReadOperation,
		Path:      pathConfigDef + "/rotate",
		Storage:   s,
	})
	assert.ErrorIs(t, err, logical.ErrUnsupportedOperation)
}

func TestConfigRotateUpdateWithoutConfig(t *testing.T) {
	b, s := getTestBackend(t)

	resp, err := b.HandleRequest(context.Background(), &logical.Request{
		Operation: logical.UpdateOperation,
		Path:      pathConfigDef + "/rotate",
		Storage:   s,
	})
	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.IsError())
}
