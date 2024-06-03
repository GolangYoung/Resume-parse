// main包是Go程序的入口点
package main

// 导入所需的库
import (
	"bytes"

	"encoding/json"

	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/ledongthuc/pdf"
)

// Resume结构体用于存储简历内容，特别是将其作为HTML模板的一部分
type Resume struct {
	Content template.HTML
}

// SaveContentAsHTML函数将HTML内容保存到文件中
func SaveContentAsHTML(content template.HTML, filename string) error {
	// 使用os.Create创建文件
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close() // 确保在函数结束时关闭文件

	// 将模板.HTML类型的内容转换为字符串并写入文件
	_, err = file.WriteString(string(content))
	if err != nil {
		return err
	}

	return nil
}

// HighlightKeywords函数用于在简历文本中高亮关键词
func HighlightKeywords(text string) string {
	// 定义一组关键词
	keywords := []string{"姓名", "专业", "电话", "邮箱", "教育背景", "个人获奖情况", "感兴趣的研究方向", "项目经历"}
	// 编译正则表达式，用于匹配关键词
	re := regexp.MustCompile(`\b(` + strings.Join(keywords, "|") + `)\b`)
	// 替换文本中的关键词为高亮格式(HTML bold标签)
	text = re.ReplaceAllString(text, `<b>$1</b>`)
	// 将文本中的换行符替换为HTML的<br>标签
	text = strings.ReplaceAll(text, "\r\n", "<br>")
	text = strings.ReplaceAll(text, "\n", "<br>")
	return text
}

// parsePDF函数用于从PDF文件中提取文本内容，并进行处理
func parsePDF(filePath string) (Resume, error) {
	// 打开PDF文件
	file, err := os.Open(filePath)
	if err != nil {
		return Resume{}, err
	}
	defer file.Close()

	// 获取文件信息
	fileInfo, err := file.Stat()
	if err != nil {
		return Resume{}, err
	}

	// 创建pdf reader
	reader, err := pdf.NewReader(file, fileInfo.Size())
	if err != nil {
		return Resume{}, err
	}

	// 创建strings.Builder用于构建提取的文本
	var text strings.Builder
	numPages := reader.NumPage()
	for i := 0; i < numPages; i++ {
		page := reader.Page(i + 1)
		content, err := page.GetPlainText(nil) // 提取页面文本
		if err != nil {
			return Resume{}, err
		}
		text.WriteString(content)
	}

	// 调用HighlightKeywords高亮文本中的关键词
	content := HighlightKeywords(text.String())
	apiKey := "sk-705d39237a4a4553900ead7c4bfde6bb"
	prompt := "帮我提取出下面这个简历的关键信息；" + content
	content, err = ParseText(apiKey, prompt) // 调用ParseText处理文本
	content = HighlightKeywords(content)     // 再次高亮以包含ParseText的输出
	log.Print(content)                       // 打印处理后的文本到日志

	return Resume{Content: template.HTML(content)}, nil
}

// uploadHandler是处理文件上传的HTTP处理器函数
func uploadHandler(c *gin.Context) {
	// 检查请求方法是否为GET
	if c.Request.Method == "GET" {
		// 如果是GET请求，渲染upload.html页面
		c.HTML(http.StatusOK, "upload.html", gin.H{})
		return
	}

	// 如果请求方法是POST，则处理文件上传
	if c.Request.Method == "POST" {
		// 解析multipart表单数据
		form, err := c.MultipartForm()
		if err != nil {
			// 如果解析表单失败，返回错误信息
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse form"})
			return
		}

		// 从表单中获取文件列表
		files := form.File["file"]
		if len(files) == 0 {
			// 如果没有文件被上传，返回错误信息
			c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
			return
		}

		// 处理第一个上传的文件
		file := files[0]
		// 创建一个临时文件存储上传的PDF文件
		tempFile, err := os.CreateTemp("", "upload-*.pdf")
		if err != nil {
			// 如果创建临时文件失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Temporary file creation failed"})
			return
		}
		defer tempFile.Close()
		defer os.Remove(tempFile.Name()) // 确保在函数结束时删除临时文件

		// 打开上传的文件内容
		src, err := file.Open()
		if err != nil {
			// 如果打开文件失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
			return
		}
		defer src.Close()

		// 将上传的文件内容复制到临时文件
		_, err = io.Copy(tempFile, src)
		if err != nil {
			// 如果文件保存失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "File saving failed"})
			return
		}

		// 调用parsePDF函数解析PDF文件
		resume, err := parsePDF(tempFile.Name())
		if err != nil {
			// 如果PDF解析失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF parsing failed"})
			return
		}

		// 将解析后的简历内容保存为HTML文件
		savedFile := "save.html"
		err = SaveContentAsHTML(resume.Content, savedFile)
		if err != nil {
			// 如果保存文件失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save result"})
			return
		}

		// 返回JSON格式的简历内容
		c.JSON(http.StatusOK, gin.H{"resume": string(resume.Content)})
	}
}

// TongYiClient 结构体包含与同义API交互所需的字段
type TongYiClient struct {
	apiKey string // API密钥
}

// TongYiRsp 结构体用于解析API的JSON响应
type TongYiRsp struct {
	Output struct { // Output 包含生成文本的相关信息
		Text         string `json:"text"`          // 生成的文本
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

// main函数是程序的入口点
func main() {

	router := gin.Default()

	// 服务静态文件，例如index.html, CSS, JavaScript等
	router.StaticFile("/", "./index.html")

	// 配置静态文件目录，例如用于服务图片或其他静态资源
	router.Static("/static", "./static")

	// 配置API端点，用于文件上传和处理
	router.POST("/api/upload", uploadHandler)

	// 捕获所有其他路由请求，并重定向到首页
	router.NoRoute(func(c *gin.Context) {
		c.File("./index.html")
	})

	log.Println("Server is running on :8080")
	router.Run(":8080")
}
