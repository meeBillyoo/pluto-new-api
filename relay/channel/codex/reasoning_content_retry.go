package codex

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var rejectedReasoningContentParamPattern = regexp.MustCompile(`^input\[(\d+)\]\.content$`)

func readAndRestoreResponseBody(resp *http.Response) ([]byte, error) {
	if resp == nil || resp.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return body, err
}

func removeRejectedReasoningContent(requestBody, responseBody []byte) ([]byte, int, bool, error) {
	if !gjson.ValidBytes(requestBody) || !gjson.ValidBytes(responseBody) {
		return nil, 0, false, nil
	}
	if gjson.GetBytes(responseBody, "error.code").String() != "array_above_max_length" {
		return nil, 0, false, nil
	}

	matches := rejectedReasoningContentParamPattern.FindStringSubmatch(
		gjson.GetBytes(responseBody, "error.param").String(),
	)
	if len(matches) != 2 {
		return nil, 0, false, nil
	}

	inputIndex, err := strconv.Atoi(matches[1])
	if err != nil || inputIndex < 0 {
		return nil, 0, false, nil
	}

	itemPath := fmt.Sprintf("input.%d", inputIndex)
	if gjson.GetBytes(requestBody, itemPath+".type").String() != "reasoning" {
		return nil, 0, false, nil
	}

	content := gjson.GetBytes(requestBody, itemPath+".content")
	if !content.Exists() || !content.IsArray() || len(content.Array()) == 0 {
		return nil, 0, false, nil
	}

	retryBody, err := sjson.DeleteBytes(requestBody, itemPath+".content")
	if err != nil {
		return nil, 0, false, err
	}
	return retryBody, inputIndex, true, nil
}
