// 定义包名
package main

// 导入所需的库
import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TongYiClient 结构体包含与同义API交互所需的字段
type TongYiClient struct {
	apiKey string // API密钥
}

// TongYiRsp 结构体用于解析API的JSON响应
type TongYiRsp struct {
	Output struct { // Output 包含生成文本的相关信息
		Text         string `json:"text"` // 生成的文本
		FinishReason string `json:"finish_reason"` // 生成完成的原因
	} `json:"output"`
	Usage struct { // Usage 包含请求的使用情况统计
		OutputTokens int `json:"output_tokens"` // 输出的token数量
		InputTokens  int `json:"input_tokens"`  // 输入的token数量
	} `json:"usage"`
	RequestID string `json:"request_id"` // 请求ID
}

// NewTongYiClient 构造函数用于创建TongYiClient实例
func NewTongYiClient(apiKey string) *TongYiClient {
	return &TongYiClient{
		apiKey: apiKey,
	}
}

// GenerateText 方法用于生成文本
func (c *TongYiClient) GenerateText(ctx context.Context, prompt string, history ...map[string]string) (*TongYiRsp, error) {
	// 创建请求数据结构
	data := map[string]interface{}{
		"model":      "qwen-turbo",
		"parameters": map[string]interface{}{},
		"input": map[string]interface{}{
			"prompt":  prompt,
			"history": history,
		},
	}

	// 将数据结构序列化为JSON
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	// 设置API请求的URL（注意：URL中的"&#34;"应替换为实际的引号，这里可能是文本复制过程中的编码问题）
	url := "https://dashscope.aliyuncs.com/api/v1/services/aigc/text-generation/generation"
	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("Content-Type", "application/json")

	// 发送HTTP请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		// 处理错误响应
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

	// 解析成功的响应
	response := &TongYiRsp{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}

	return response, nil
}

// ParseText 函数用于解析文本内容并调用同义API生成文本
func ParseText(apiKey, prompt string, history ...map[string]string) (string, error) {
	client := NewTongYiClient(apiKey) // 创建客户端实例

	// 生成文本
	response, err := client.GenerateText(context.Background(), prompt, history...)
	if err != nil {
		return "", fmt.Errorf("failed to generate text: %v", err)
	}

	// 检查生成的文本是否为空
	if response.Output.Text == "" {
		return "", fmt.Errorf("generated text is empty")
	}

	// 返回生成的文本
	return response.Output.Text, nil
}