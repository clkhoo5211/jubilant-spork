package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// Provider AI提供商类型
type Provider string

const (
	ProviderDeepSeek   Provider = "deepseek"
	ProviderQwen       Provider = "qwen"
	ProviderCustom     Provider = "custom"
	ProviderGemini     Provider = "gemini"
	ProviderHuggingFace Provider = "huggingface"
)

// Client AI API配置
type Client struct {
	Provider   Provider
	APIKey     string
	SecretKey  string // 阿里云需要
	BaseURL    string
	Model      string
	Timeout    time.Duration
	UseFullURL bool // 是否使用完整URL（不添加/chat/completions）
}

func New() *Client {
	// 默认配置
	var defaultClient = Client{
		Provider: ProviderDeepSeek,
		BaseURL:  "https://api.deepseek.com/v1",
		Model:    "deepseek-chat",
		Timeout:  120 * time.Second, // 增加到120秒，因为AI需要分析大量数据
	}
	return &defaultClient
}

// SetDeepSeekAPIKey 设置DeepSeek API密钥
func (cfg *Client) SetDeepSeekAPIKey(apiKey string) {
	cfg.Provider = ProviderDeepSeek
	cfg.APIKey = apiKey
	cfg.BaseURL = "https://api.deepseek.com/v1"
	cfg.Model = "deepseek-chat"
}

// SetQwenAPIKey 设置阿里云Qwen API密钥
func (cfg *Client) SetQwenAPIKey(apiKey, secretKey string) {
	cfg.Provider = ProviderQwen
	cfg.APIKey = apiKey
	cfg.SecretKey = secretKey
	cfg.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	cfg.Model = "qwen-plus" // 可选: qwen-turbo, qwen-plus, qwen-max
}

// SetCustomAPI 设置自定义OpenAI兼容API
func (cfg *Client) SetCustomAPI(apiURL, apiKey, modelName string) {
	cfg.Provider = ProviderCustom
	cfg.APIKey = apiKey

	// 检测是否为Google Gemini API
	if strings.Contains(apiURL, "generativelanguage.googleapis.com") {
		cfg.Provider = ProviderGemini
		cfg.BaseURL = apiURL
		cfg.UseFullURL = true // Gemini使用自定义端点
	} else if strings.Contains(apiURL, "router.huggingface.co") || strings.Contains(apiURL, "api-inference.huggingface.co") || strings.Contains(apiURL, "huggingface.co") {
		// 检测是否为Hugging Face Inference API
		cfg.Provider = ProviderHuggingFace
		cfg.BaseURL = apiURL
		cfg.UseFullURL = true // Hugging Face使用自定义端点
	} else if strings.HasSuffix(apiURL, "#") {
		// 检查URL是否以#结尾，如果是则使用完整URL（不添加/chat/completions）
		cfg.BaseURL = strings.TrimSuffix(apiURL, "#")
		cfg.UseFullURL = true
	} else {
		cfg.BaseURL = apiURL
		cfg.UseFullURL = false
	}

	cfg.Model = modelName
	cfg.Timeout = 120 * time.Second
}

// SetClient 设置完整的AI配置（高级用户）
func (cfg *Client) SetClient(Client Client) {
	if Client.Timeout == 0 {
		Client.Timeout = 30 * time.Second
	}
	cfg = &Client
}

// CallWithMessages 使用 system + user prompt 调用AI API（推荐）
func (cfg *Client) CallWithMessages(systemPrompt, userPrompt string) (string, error) {
	if cfg.APIKey == "" {
		return "", fmt.Errorf("AI API密钥未设置，请先调用 SetDeepSeekAPIKey() 或 SetQwenAPIKey()")
	}

	// 重试配置
	maxRetries := 3
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			fmt.Printf("⚠️  AI API调用失败，正在重试 (%d/%d)...\n", attempt, maxRetries)
		}

		result, err := cfg.callOnce(systemPrompt, userPrompt)
		if err == nil {
			if attempt > 1 {
				fmt.Printf("✓ AI API重试成功\n")
			}
			return result, nil
		}

		lastErr = err
		// 如果不是网络错误，不重试
		if !isRetryableError(err) {
			return "", err
		}

		// 重试前等待
		if attempt < maxRetries {
			waitTime := time.Duration(attempt) * 2 * time.Second
			fmt.Printf("⏳ 等待%v后重试...\n", waitTime)
			time.Sleep(waitTime)
		}
	}

	return "", fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// callOnce 单次调用AI API（内部使用）
func (cfg *Client) callOnce(systemPrompt, userPrompt string) (string, error) {
	// 如果是Gemini API，使用特殊的请求格式
	if cfg.Provider == ProviderGemini {
		return cfg.callGeminiAPI(systemPrompt, userPrompt)
	}

	// 如果是Hugging Face API，使用特殊的请求格式
	if cfg.Provider == ProviderHuggingFace {
		return cfg.callHuggingFaceAPI(systemPrompt, userPrompt)
	}

	// 构建 messages 数组
	messages := []map[string]string{}

	// 如果有 system prompt，添加 system message
	if systemPrompt != "" {
		messages = append(messages, map[string]string{
			"role":    "system",
			"content": systemPrompt,
		})
	}

	// 添加 user message
	messages = append(messages, map[string]string{
		"role":    "user",
		"content": userPrompt,
	})

	// 构建请求体
	requestBody := map[string]interface{}{
		"model":       cfg.Model,
		"messages":    messages,
		"temperature": 0.5, // 降低temperature以提高JSON格式稳定性
		"max_tokens":  8000, // 增加token限制以容纳长思维链和JSON决策
	}

	// 注意：response_format 参数仅 OpenAI 支持，DeepSeek/Qwen 不支持
	// 我们通过强化 prompt 和后处理来确保 JSON 格式正确

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	// 创建HTTP请求
	var url string
	if cfg.UseFullURL {
		// 使用完整URL，不添加/chat/completions
		url = cfg.BaseURL
	} else {
		// 默认行为：添加/chat/completions
		url = fmt.Sprintf("%s/chat/completions", cfg.BaseURL)
	}
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// 根据不同的Provider设置认证方式
	switch cfg.Provider {
	case ProviderDeepSeek:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	case ProviderQwen:
		// 阿里云Qwen使用API-Key认证
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
		// 注意：如果使用的不是兼容模式，可能需要不同的认证方式
	default:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	}

	// 发送请求
	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析响应
	var result struct {
		Choices []struct {
			Message struct {
				Content   string `json:"content"`
				Reasoning string `json:"reasoning"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("API返回空响应")
	}

	content := result.Choices[0].Message.Content
	reasoning := result.Choices[0].Message.Reasoning
	
	// 如果content为空，尝试使用reasoning字段（用于支持reasoning模式的模型，如Qwen3）
	if content == "" || content == " " || content == "<s>" || content == "<s> " {
		if reasoning != "" && reasoning != " " {
			content = reasoning
		}
	}
	
	// 清理响应内容（移除<s>等标记）
	content = cleanResponse(content)
	
	if content == "" || content == " " || content == "<s>" || content == "<s> " {
		// 如果内容为空或只有标记，返回一个默认的成功响应
		return "{\"decision\": \"hold\", \"reasoning\": \"AI模型返回空响应，建议保持观望\"}", nil
	}

	return content, nil
}

// isRetryableError 判断错误是否可重试
func isRetryableError(err error) bool {
	errStr := err.Error()
	// 网络错误、超时、EOF等可以重试
	retryableErrors := []string{
		"EOF",
		"timeout",
		"connection reset",
		"connection refused",
		"temporary failure",
		"no such host",
	}
	for _, retryable := range retryableErrors {
		if strings.Contains(errStr, retryable) {
			return true
		}
	}
	return false
}

// callGeminiAPI 调用Google Gemini API（使用Gemini特定的API格式）
func (cfg *Client) callGeminiAPI(systemPrompt, userPrompt string) (string, error) {
    // Gemini API端点: /v1beta/models/{model}:generateContent
    // 使用请求头 x-goog-api-key 传递密钥（参考官方文档）
    url := fmt.Sprintf("%s/models/%s:generateContent", cfg.BaseURL, cfg.Model)

	// 构建Gemini API格式的请求
	// Gemini使用contents数组而不是messages
	var contents []map[string]interface{}

	// 合并system和user prompt
	fullContent := ""
	if systemPrompt != "" {
		fullContent = systemPrompt + "\n\n" + userPrompt
	} else {
		fullContent = userPrompt
	}

	contents = append(contents, map[string]interface{}{
		"role": "user",
		"parts": []map[string]string{
			{"text": fullContent},
		},
	})

	requestBody := map[string]interface{}{
		"contents": contents,
		"generationConfig": map[string]interface{}{
			"temperature":     0.5,
			"maxOutputTokens": 8192, // 增加token限制以容纳模型的内部推理和实际输出
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化Gemini请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建Gemini请求失败: %w", err)
	}

    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-goog-api-key", cfg.APIKey)

	// 发送请求
	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送Gemini请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取Gemini响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Gemini API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析Gemini响应格式
	var geminiResult struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
			FinishReason string `json:"finishReason"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(body, &geminiResult); err != nil {
		// 如果解析失败，返回原始响应以便调试
		return "", fmt.Errorf("解析Gemini响应失败: %w, 响应内容: %s", err, string(body))
	}

	if len(geminiResult.Candidates) == 0 {
		// 检查是否有finishReason等信息
		return "", fmt.Errorf("Gemini API返回空响应，原始响应: %s", string(body))
	}

	if len(geminiResult.Candidates[0].Content.Parts) == 0 {
		// 检查finishReason，如果是MAX_TOKENS，提示需要增加maxOutputTokens
		if geminiResult.Candidates[0].FinishReason == "MAX_TOKENS" {
			return "", fmt.Errorf("Gemini API达到token限制 (finishReason=MAX_TOKENS)，响应可能被截断。请缩短思维链分析，优先保证JSON数组输出。原始响应: %s", string(body))
		}
		return "", fmt.Errorf("Gemini API返回空内容 (finishReason=%s)，原始响应: %s", geminiResult.Candidates[0].FinishReason, string(body))
	}

	text := geminiResult.Candidates[0].Content.Parts[0].Text
	
	// 检查是否因为MAX_TOKENS而截断
	if geminiResult.Candidates[0].FinishReason == "MAX_TOKENS" {
		log.Printf("⚠️ 警告: Gemini响应达到token限制，响应可能被截断。响应长度: %d字符", len(text))
		// 即使被截断也返回文本，让解析逻辑尝试提取JSON
	}
	
	return text, nil
}

// callHuggingFaceAPI 调用Hugging Face Inference API
func (cfg *Client) callHuggingFaceAPI(systemPrompt, userPrompt string) (string, error) {
	// 检测是否为新版 Inference Providers API (OpenAI兼容格式)
	isNewAPI := strings.Contains(cfg.BaseURL, "router.huggingface.co")
	
	var url string
	if isNewAPI {
		// 新版 API: 使用 OpenAI 兼容格式 /v1/chat/completions
		url = fmt.Sprintf("%s/v1/chat/completions", cfg.BaseURL)
	} else {
		// 旧版 API: /models/{model_name}
		if strings.Contains(cfg.BaseURL, "/models/") {
			url = cfg.BaseURL
		} else {
			url = fmt.Sprintf("%s/models/%s", cfg.BaseURL, cfg.Model)
		}
	}

	// 如果是新版 API，使用 OpenAI 兼容格式
	if isNewAPI {
		// 构建 OpenAI 兼容的请求格式
		messages := []map[string]string{}
		if systemPrompt != "" {
			messages = append(messages, map[string]string{
				"role":    "system",
				"content": systemPrompt,
			})
		}
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": userPrompt,
		})

		requestBody := map[string]interface{}{
			"model":       cfg.Model,
			"messages":    messages,
			"temperature": 0.5,
			"max_tokens":  8000,
		}

		jsonData, err := json.Marshal(requestBody)
		if err != nil {
			return "", fmt.Errorf("序列化Hugging Face请求失败: %w", err)
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			return "", fmt.Errorf("创建Hugging Face请求失败: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

		client := &http.Client{Timeout: cfg.Timeout}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("发送Hugging Face请求失败: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("读取Hugging Face响应失败: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == 503 || resp.StatusCode == 202 {
				return "", fmt.Errorf("Hugging Face模型正在加载中，请稍后重试 (status %d)", resp.StatusCode)
			}
			return "", fmt.Errorf("Hugging Face API返回错误 (status %d): %s", resp.StatusCode, string(body))
		}

		// 解析 OpenAI 兼容格式的响应
		var result struct {
			Choices []struct {
				Message struct {
					Content   string `json:"content"`
					Reasoning string `json:"reasoning"`
				} `json:"message"`
			} `json:"choices"`
		}

		if err := json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("解析Hugging Face响应失败: %w", err)
		}

		if len(result.Choices) == 0 {
			return "", fmt.Errorf("Hugging Face API返回空响应")
		}

		content := result.Choices[0].Message.Content
		reasoning := result.Choices[0].Message.Reasoning
		
		// 如果content为空，尝试使用reasoning字段
		if content == "" || content == " " {
			if reasoning != "" && reasoning != " " {
				content = reasoning
			}
		}

		return content, nil
	}

	// 旧版 API 格式处理
	// 构建Hugging Face API格式的请求
	// 合并system和user prompt
	fullContent := ""
	if systemPrompt != "" {
		fullContent = systemPrompt + "\n\n" + userPrompt
	} else {
		fullContent = userPrompt
	}

	requestBody := map[string]interface{}{
		"inputs": fullContent,
		"parameters": map[string]interface{}{
			"temperature":     0.5,
			"max_new_tokens":  2000,
			"return_full_text": false,
		},
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("序列化Hugging Face请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("创建Hugging Face请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))

	// 发送请求
	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("发送Hugging Face请求失败: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取Hugging Face响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Hugging Face可能返回202 (model is loading)，需要等待
		if resp.StatusCode == 503 || resp.StatusCode == 202 {
			return "", fmt.Errorf("Hugging Face模型正在加载中，请稍后重试 (status %d)", resp.StatusCode)
		}
		return "", fmt.Errorf("Hugging Face API返回错误 (status %d): %s", resp.StatusCode, string(body))
	}

	// 解析Hugging Face响应格式
	// Hugging Face返回格式可能是数组或单个对象
	var hfResult []map[string]interface{}
	var hfResultSingle map[string]interface{}

	// 尝试解析为数组
	if err := json.Unmarshal(body, &hfResult); err == nil && len(hfResult) > 0 {
		// 数组格式
		if generatedText, ok := hfResult[0]["generated_text"].(string); ok {
			// 移除原始输入（Hugging Face返回完整文本）
			if strings.HasPrefix(generatedText, fullContent) {
				return strings.TrimPrefix(generatedText, fullContent), nil
			}
			return generatedText, nil
		}
	}

	// 尝试解析为单个对象
	if err := json.Unmarshal(body, &hfResultSingle); err == nil {
		if generatedText, ok := hfResultSingle["generated_text"].(string); ok {
			if strings.HasPrefix(generatedText, fullContent) {
				return strings.TrimPrefix(generatedText, fullContent), nil
			}
			return generatedText, nil
		}
	}

	// 如果以上都失败，尝试作为字符串数组处理
	var textArray []string
	if err := json.Unmarshal(body, &textArray); err == nil && len(textArray) > 0 {
		return textArray[0], nil
	}

	return "", fmt.Errorf("无法解析Hugging Face响应: %s", string(body))
}

// cleanResponse 清理AI响应内容，移除特殊标记
func cleanResponse(content string) string {
	// 移除常见的模型标记
	markers := []string{"<s>", "</s>", "[INST]", "[/INST]", "<|im_start|>", "<|im_end|>"}
	
	for _, marker := range markers {
		content = strings.ReplaceAll(content, marker, "")
	}
	
	// 移除多余的空格和换行
	content = strings.TrimSpace(content)
	
	return content
}
