// main包是Go程序的入口点
package main

// 导入所需的库
import (
	"net/http"

	"html/template"
	"io"
	"log"
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
	content = HighlightKeywords(content) // 再次高亮以包含ParseText的输出
	log.Print(content) // 打印处理后的文本到日志

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
		savedFile := "resume_output.html"
		err = SaveContentAsHTML(resume.Content, savedFile)
		if err != nil {
			// 如果保存文件失败，返回错误信息
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save result"})
			return
		}

		// 渲染result.html页面，并传递解析后的简历数据和保存的文件名
		c.HTML(http.StatusOK, "result.html", gin.H{"resume": resume, "savedFile": savedFile})
	}
}

// main函数是程序的入口点
func main() {
	// 创建一个默认配置的Gin路由器实例
	router := gin.Default()
	// 加载所有templates目录下的HTML模板文件
	router.LoadHTMLGlob("template/*")
	// 设置静态文件目录
	router.Static("/static", "./static")
	// 设置/upload路由的处理函数为uploadHandler
	router.POST("/upload", uploadHandler)
	router.GET("/upload", uploadHandler)
	// 打印服务器正在监听的端口信息
	log.Println("Server is running on :8080")
	// 运行Gin路由器，监听并服务于8080端口
	router.Run(":8080")
}