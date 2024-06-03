package main

import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/unidoc/unidoc/pdf/extractor"
	"github.com/unidoc/unidoc/pdf/model"
)

// Resume 结构体用于存储简历内容
type Resume struct {
	Content template.HTML
}

// SaveContentAsHTML 函数用于将HTML内容保存为HTML文件
func SaveContentAsHTML(content template.HTML, filename string) error {
	// 创建或打开文件用于写入
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// 直接写入HTML内容到文件，无需转换为字符串
	_, err = file.WriteString(string(content))
	if err != nil {
		return err
	}

	return nil
}

// HighlightKeywords 函数用于高亮简历中的关键词
func HighlightKeywords(text string) string {
	keywords := []string{"姓名", "专业", "电话", "邮箱", "教育背景", "个人获奖情况", "感兴趣的研究方向", "项目经历"}
	for _, keyword := range keywords {
		// 使用正则表达式的边界匹配，确保完全匹配关键词
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `\b`)
		text = re.ReplaceAllString(text, `<b>$&</b>`) // $& 表示整个匹配的内容
	}
	// 将换行符替换为HTML的<br>标签
	text = strings.ReplaceAll(text, "\r\n", "<br>")
	text = strings.ReplaceAll(text, "\n", "<br>")
	return text
}

// parsePDF 函数用于从指定路径的PDF文件中提取文本内容，并进行处理。
func parsePDF(filePath string) (Resume, error) {
	// 打开PDF文件
	f, err := os.Open(filePath)
	if err != nil {
		// 如果打开文件失败，返回错误并退出函数
		return Resume{}, err
	}
	defer f.Close() // 确保在函数结束时关闭文件

	// 创建一个pdfReader对象，用于读取PDF文件
	pdfReader, err := model.NewPdfReader(f)
	if err != nil {
		// 如果创建pdfReader失败，返回错误并退出函数
		return Resume{}, err
	}

	// 创建一个strings.Builder对象，用于构建提取的文本
	var extractedText strings.Builder
	// 获取PDF文件的总页数
	numPages, err := pdfReader.GetNumPages()
	if err != nil {
		// 如果获取页数失败，返回错误并退出函数
		return Resume{}, err
	}

	// 遍历PDF的每一页
	for i := 1; i <= numPages; i++ {
		// 获取当前页
		page, err := pdfReader.GetPage(i)
		if err != nil {
			// 如果获取页面失败，返回错误并退出函数
			return Resume{}, err
		}

		// 创建一个文本提取器
		textExtractor, err := extractor.New(page)
		if err != nil {
			// 如果创建文本提取器失败，返回错误并退出函数
			return Resume{}, err
		}

		// 从当前页提取文本
		text, err := textExtractor.ExtractText()
		log.Println(text) // 打印提取的文本（实际使用中可能不需要这行）

		// 以下代码片段是示例性质的，实际代码中应根据具体逻辑处理
		// 假设有一个ParseText函数，用于处理提取的文本
		// apiKey是某个API的密钥，prompt是要发送给API的提示信息
		apiKey := "sk-705d39237a4a4553900ead7c4bfde6bb"
		prompt := "帮我提取出下面这个简历的关键信息；" + text
		text, err = ParseText(apiKey, prompt) // 调用ParseText处理文本

		if err != nil {
			// 如果处理文本失败，返回错误并退出函数
			return Resume{}, err
		}

		// 将处理后的文本添加到提取文本构建器中
		extractedText.WriteString(text)
	}

	// 调用highlightKeywords函数高亮显示提取的文本中的关键词
	content := HighlightKeywords(extractedText.String())

	// 将处理后的HTML内容保存到文件
	savedFile := "resume_output.html" // 定义保存的文件名
	err = SaveContentAsHTML(template.HTML(content), savedFile)
	if err != nil {
		log.Println("Error saving content as HTML file:", err)
		// 如果保存文件失败，记录错误日志，可以选择返回错误或者继续
	}

	// 返回Resume结构体，包含处理后的HTML内容
	return Resume{Content: template.HTML(content)}, nil
}

// uploadHandler 是处理文件上传和结果显示的HTTP处理器函数
func uploadHandler(c *gin.Context) {
	// 检查请求方法是否为GET
	if c.Request.Method == "GET" {
		// 如果是GET请求，渲染并返回upload.html页面
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

		// 创建一个临时文件，用于存储上传的PDF文件
		tempFile, err := os.CreateTemp("", "upload-*.pdf")
		if err != nil {
			// 如果创建临时文件失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Temporary file creation failed"})
			return
		}
		defer tempFile.Close()           // 确保在函数结束时关闭临时文件
		defer os.Remove(tempFile.Name()) // 确保在函数结束时删除临时文件

		// 打开上传的文件内容
		fileContent, err := file.Open()
		if err != nil {
			// 如果打开文件失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
			return
		}
		defer fileContent.Close() // 确保在函数结束时关闭文件内容

		// 将上传的文件内容复制到临时文件
		_, err = io.Copy(tempFile, fileContent)
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

		// 渲染result.html页面，并传递解析后的简历数据
		c.HTML(http.StatusOK, "result.html", gin.H{"resume": resume})
	}
}

func main() {
	// 创建一个默认配置的Gin路由器实例
	router := gin.Default()

	// 加载所有templates目录下的HTML模板文件
	router.LoadHTMLGlob("templates/*")

	// 设置静态文件目录，用于提供静态资源如CSS、JavaScript等
	router.Static("/static", "./static")

	// 设置/upload路由的处理函数为uploadHandler
	// POST请求用于文件上传，GET请求用于显示上传页面
	router.POST("/upload", uploadHandler)
	router.GET("/upload", uploadHandler)

	// 打印服务器正在监听的端口信息
	log.Println("Server is running on :8080")
	// 运行Gin路由器，监听并服务于8080端口
	if err := router.Run(":8080"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
