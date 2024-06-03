package controllers

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/astaxie/beego"
	"github.com/ledongthuc/pdf"
)

func init() {
	// 注册 safeHTML 函数到 beego 模板引擎
	beego.AddFuncMap("safeHTML", func(text string) template.HTML {
		return template.HTML(text)
	})
}

// Resume struct to store resume content
type Resume struct {
	Content string // 改为 string 类型
}

// SaveContentAsHTML function to save HTML content as HTML file
func SaveContentAsHTML(content template.HTML, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(string(content))
	if err != nil {
		return err
	}

	return nil
}

// HighlightKeywords function to highlight keywords in resume
func HighlightKeywords(text string) string {
	keywords := []string{"姓名", "专业", "电话", "邮箱", "教育背景", "个人获奖情况", "感兴趣的研究方向", "项目经历"}
	re := regexp.MustCompile(`(` + strings.Join(keywords, "|") + `)`)
	text = re.ReplaceAllString(text, `<b>$1</b>`)
	text = strings.ReplaceAll(text, "\r\n", "<br>")
	text = strings.ReplaceAll(text, "\n", "<br>")
	return text
}

// ParsePDF 函数用于从PDF文件中提取和处理文本内容
func ParsePDF(filePath string) (Resume, error) {
	// 打开文件
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

	// 创建PDF阅读器
	reader, err := pdf.NewReader(file, fileInfo.Size())
	if err != nil {
		return Resume{}, err
	}

	// 用于存储PDF文本内容的字符串构建器
	var text strings.Builder
	numPages := reader.NumPage()
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := reader.Page(pageNum)
		// 获取页面的纯文本内容
		content, err := page.GetPlainText(nil)
		if err != nil {
			return Resume{}, err
		}
		text.WriteString(content)
		if pageNum < numPages {
			text.WriteString("\n--- 分页符 ---\n")
		}
	}

	// 对文本内容进行关键字高亮处理
	content := HighlightKeywords(text.String())
	log.Print(content)

	// 调用 ParseText 函数处理文本
	apiKey := "sk-705d39237a4a4553900ead7c4bfde6bb"
	prompt := "帮我提取出下面这个简历的关键信息:" + content
	content, err = ParseText(apiKey, prompt)
	content = HighlightKeywords(content)

	// 返回解析后的简历内容
	return Resume{Content: content}, nil // Content 保持为 string 类型
}

// PDFController 用于处理PDF文件的上传和解析
type PDFController struct {
	beego.Controller
}

// Get 方法用于显示上传页面
func (c *PDFController) Get() {
	c.TplName = "upload.html"
}

// Post 方法用于处理PDF文件的上传和解析
func (c *PDFController) Post() {
	file, _, err := c.GetFile("file")
	if err != nil {
		c.Ctx.WriteString("获取上传文件失败：" + err.Error())
		return
	}
	defer file.Close()

	// 创建临时文件用于存储上传的PDF文件内容
	tempFile, err := os.CreateTemp("", "*.pdf")
	if err != nil {
		c.Ctx.WriteString("创建临时文件失败：" + err.Error())
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// 将上传的文件内容复制到临时文件中
	_, err = io.Copy(tempFile, file)
	if err != nil {
		c.Ctx.WriteString("保存上传文件失败：" + err.Error())
		return
	}

	// 验证文件是否为有效的PDF格式，通过检查文件开头是否包含"%PDF-"
	if err := ValidatePDF(tempFile); err != nil {
		c.Ctx.WriteString("上传文件不是有效的PDF：" + err.Error())
		return
	}

	// 解析PDF文件内容
	resume, err := ParsePDF(tempFile.Name())
	if err != nil {
		c.Ctx.WriteString("解析PDF失败：" + err.Error())
		return
	}

	// 将解析后的内容传递给模板
	c.Data["ContentForTemplate"] = resume.Content

	// 渲染模板
	c.TplName = "result.html"
	c.Layout = "layout.html"
	c.Render()
}

// ValidatePDF 函数用于检查文件是否为有效的PDF格式
func ValidatePDF(file *os.File) error {
	// 将文件指针移到文件开头
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}

	// 读取文件的前5个字节，检查是否以"%PDF-"开头
	buf := make([]byte, 5)
	_, err = file.Read(buf)
	if err != nil {
		return err
	}

	if string(buf) != "%PDF-" {
		return fmt.Errorf("文件不是以 %PDF- 开头")
	}

	return nil
}
