package codex

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestGetRequestURLAlphaSearch(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: "https://chatgpt.com",
		},
		RelayMode: relayconstant.RelayModeAlphaSearch,
	}

	url, err := adaptor.GetRequestURL(info)
	require.NoError(t, err)
	assert.Equal(t, "https://chatgpt.com/backend-api/codex/alpha/search", url)
}

// The Codex backend rejects these fields, so the adaptor clears them rather
// than forwarding what the client sent.
func TestConvertOpenAIResponsesRequestDropsPenalties(t *testing.T) {
	adaptor := &Adaptor{}
	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{ChannelType: constant.ChannelTypeCodex},
		RelayMode:   relayconstant.RelayModeResponses,
	}

	converted, err := adaptor.ConvertOpenAIResponsesRequest(nil, info, dto.OpenAIResponsesRequest{
		Model:            "gpt-5-codex",
		Input:            json.RawMessage(`"hello"`),
		MaxOutputTokens:  lo.ToPtr(uint(128)),
		Temperature:      lo.ToPtr(1.0),
		FrequencyPenalty: json.RawMessage(`1.5`),
		PresencePenalty:  json.RawMessage(`1.5`),
	})
	require.NoError(t, err)

	request, ok := converted.(dto.OpenAIResponsesRequest)
	require.True(t, ok)
	assert.Nil(t, request.MaxOutputTokens)
	assert.Nil(t, request.Temperature)
	assert.Nil(t, request.FrequencyPenalty)
	assert.Nil(t, request.PresencePenalty)
}

func TestDoRequestRetriesAfterRejectedReasoningContent(t *testing.T) {
	var requestBodies [][]byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		requestBodies = append(requestBodies, body)

		if len(requestBodies) == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, err = w.Write([]byte(`{"error":{"code":"array_above_max_length","param":"input[1].content"}}`))
			require.NoError(t, err)
			return
		}

		w.WriteHeader(http.StatusOK)
		_, err = w.Write([]byte(`{"output":[]}`))
		require.NoError(t, err)
	}))
	defer upstream.Close()

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{}`))

	info := &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    constant.ChannelTypeCodex,
			ChannelBaseUrl: upstream.URL,
			ApiKey:         `{"access_token":"access-token","account_id":"account-id"}`,
		},
		IsStream:  false,
		RelayMode: relayconstant.RelayModeResponses,
	}
	requestBody, closer, err := relaycommon.NewOutboundJSONBody([]byte(`{"input":[{"type":"message","content":[{"type":"input_text","text":"hello"}]},{"type":"reasoning","summary":[],"content":[{"type":"reasoning_text","text":"remove"}]}]}`))
	require.NoError(t, err)
	defer closer.Close()

	result, err := (&Adaptor{}).DoRequest(ctx, info, requestBody)

	require.NoError(t, err)
	response, ok := result.(*http.Response)
	require.True(t, ok)
	require.Len(t, requestBodies, 2)
	assert.Equal(t, http.StatusOK, response.StatusCode)
	assert.True(t, gjson.GetBytes(requestBodies[0], "input.1.content").Exists())
	assert.False(t, gjson.GetBytes(requestBodies[1], "input.1.content").Exists())
}
