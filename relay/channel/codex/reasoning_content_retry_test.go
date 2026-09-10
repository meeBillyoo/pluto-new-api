package codex

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRemoveRejectedReasoningContent(t *testing.T) {
	requestBody := []byte(`{"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"keep user"}]},{"type":"reasoning","id":"rs_1","summary":[{"type":"summary_text","text":"keep summary"}],"encrypted_content":"keep encrypted","content":[{"type":"reasoning_text","text":"remove reasoning"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"keep assistant"}]}]}`)
	responseBody := []byte(`{"error":{"code":"array_above_max_length","param":"input[1].content","type":"invalid_request_error"}}`)

	retryBody, inputIndex, shouldRetry, err := removeRejectedReasoningContent(requestBody, responseBody)

	require.NoError(t, err)
	require.True(t, shouldRetry)
	assert.Equal(t, 1, inputIndex)
	assert.False(t, gjson.GetBytes(retryBody, "input.1.content").Exists())
	assert.Equal(t, "rs_1", gjson.GetBytes(retryBody, "input.1.id").String())
	assert.Equal(t, "keep summary", gjson.GetBytes(retryBody, "input.1.summary.0.text").String())
	assert.Equal(t, "keep encrypted", gjson.GetBytes(retryBody, "input.1.encrypted_content").String())
	assert.Equal(t, "keep user", gjson.GetBytes(retryBody, "input.0.content.0.text").String())
	assert.Equal(t, "keep assistant", gjson.GetBytes(retryBody, "input.2.content.0.text").String())
}

func TestRemoveRejectedReasoningContentRejectsAmbiguousErrors(t *testing.T) {
	tests := []struct {
		name         string
		requestBody  string
		responseBody string
	}{
		{
			name:         "wrong error code",
			requestBody:  `{"input":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"keep"}]}]}`,
			responseBody: `{"error":{"code":"invalid_request_error","param":"input[0].content"}}`,
		},
		{
			name:         "missing parameter",
			requestBody:  `{"input":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"keep"}]}]}`,
			responseBody: `{"error":{"code":"array_above_max_length"}}`,
		},
		{
			name:         "message item",
			requestBody:  `{"input":[{"type":"message","content":[{"type":"input_text","text":"keep"}]}]}`,
			responseBody: `{"error":{"code":"array_above_max_length","param":"input[0].content"}}`,
		},
		{
			name:         "function call item",
			requestBody:  `{"input":[{"type":"function_call","content":[{"type":"input_text","text":"keep"}]}]}`,
			responseBody: `{"error":{"code":"array_above_max_length","param":"input[0].content"}}`,
		},
		{
			name:         "empty content",
			requestBody:  `{"input":[{"type":"reasoning","content":[]}]}`,
			responseBody: `{"error":{"code":"array_above_max_length","param":"input[0].content"}}`,
		},
		{
			name:         "out of range index",
			requestBody:  `{"input":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"keep"}]}]}`,
			responseBody: `{"error":{"code":"array_above_max_length","param":"input[1].content"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retryBody, inputIndex, shouldRetry, err := removeRejectedReasoningContent(
				[]byte(tt.requestBody),
				[]byte(tt.responseBody),
			)

			require.NoError(t, err)
			assert.False(t, shouldRetry)
			assert.Zero(t, inputIndex)
			assert.Nil(t, retryBody)
		})
	}
}

func TestReadAndRestoreResponseBody(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"error":"test"}`)),
	}

	body, err := readAndRestoreResponseBody(resp)

	require.NoError(t, err)
	assert.Equal(t, `{"error":"test"}`, string(body))

	restored, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, body, restored)
}
