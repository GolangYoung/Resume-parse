package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type TongYiClient struct {
	apiKey string
}

type TongYiRsp struct {
	Output struct {
		Text         string `json:"text"`
		FinishReason string `json:"finish_reason"`
	} `json:"output"`
	Usage struct {
		OutputTokens int `json:"output_tokens"`
		InputTokens  int `json:"input_tokens"`
	} `json:"usage"`
	RequestID string `json:"request_id"`
}

func NewTongYiClient(apiKey string) *TongYiClient {
	return &TongYiClient{
		apiKey: apiKey,
	}
}

func (c *TongYiClient) GenerateText(ctx context.Context, prompt string, history ...map[string]string) (*TongYiRsp, error) {
	data := map[string]interface{}{
		"model":      "qwen-turbo",
		"parameters": map[string]interface{}{},
		"input": map[string]interface{}{
			"prompt":  prompt,
			"history": history,
		},
	}

	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errorResponse struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		}

		err = json.NewDecoder(resp.Body).Decode(&errorResponse)
		if err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("API error: %s - %s", errorResponse.Code, errorResponse.Message)
	}

	response := &TongYiRsp{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// ParseText 解析文字内容
func ParseText(apiKey, prompt string, history ...map[string]string) (string, error) {
	client := NewTongYiClient(apiKey)

	response, err := client.GenerateText(context.Background(), prompt, history...)
	if err != nil {
		return "", fmt.Errorf("failed to generate text: %v", err)
	}

	if response.Output.Text == "" {
		return "", fmt.Errorf("generated text is empty")
	}

	return response.Output.Text, nil
}

