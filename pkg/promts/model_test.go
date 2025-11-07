package prompts_test

import (
	"testing"

	"pod_api/pkg/promts"

	"github.com/mailru/easyjson"
	"github.com/stretchr/testify/require"
)

func TestSerialization(t *testing.T) {
	msg := prompts.Message{
		Role:    prompts.RoleUser,
		Content: "Test content.",
	}

	data, err := easyjson.Marshal(msg)
	require.NoError(t, err)

	var decoded prompts.Message
	err = easyjson.Unmarshal(data, &decoded)
	require.NoError(t, err)

	require.Equal(t, decoded.Role, msg.Role)
	require.Equal(t, decoded.Content, msg.Content)
}
