package main

// 到入必要的包
import (
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/unidoc/unidoc/pdf/extractor"
	"github.com/unidoc/unidoc/pdf/model"
)

// Resume 结构体用于存储简历内容
type Resume struct {
	Content template.HTML
}

// highlightKeywords 函数用于高亮简历中的关键词
func highlightKeywords(text string) string {
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

// saveContentAsHTML 函数用于将HTML内容保存为HTML文件
func saveContentAsHTML(content template.HTML, filename string) error {
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
	content := highlightKeywords(extractedText.String())

	// 将处理后的HTML内容保存到文件
	savedFile := "resume_output.html" // 定义保存的文件名
	err = saveContentAsHTML(template.HTML(content), savedFile)
	if err != nil {
		log.Println("Error saving content as HTML file:", err)
		// 如果保存文件失败，记录错误日志，可以选择返回错误或者继续
	}

	// 返回Resume结构体，包含处理后的HTML内容
	return Resume{Content: template.HTML(content)}, nil
}

// uploadHandler 处理文件上传和结果显示
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// 如果是GET请求，加载并显示上传表单
	if r.Method == http.MethodGet {
		tmpl, err := template.ParseFiles("upload.html") // 尝试解析上传表单的HTML模板
		if err != nil {
			http.Error(w, "模板加载失败", http.StatusInternalServerError) // 模板加载出错时返回500状态码
			return
		}
		tmpl.Execute(w, nil) // 渲染并发送表单到客户端
		return
	}

	// 如果是POST请求，处理文件上传
	if r.Method == http.MethodPost {
		file, _, err := r.FormFile("file") // 从表单中获取名为"file"的文件
		if err != nil {
			http.Error(w, "文件上传错误", http.StatusBadRequest) // 文件读取错误时返回400状态码
			return
		}
		defer file.Close() // 确保文件句柄在操作完成后关闭

		// 创建一个临时文件来存储上传的PDF
		tempFile, err := os.CreateTemp("", "upload-*.pdf")
		if err != nil {
			http.Error(w, "创建临时文件失败", http.StatusInternalServerError) // 创建临时文件失败时返回500状态码
			return
		}
		defer os.Remove(tempFile.Name()) // 上传处理后删除临时文件

		// 将上传的文件内容复制到临时文件
		_, err = io.Copy(tempFile, file)
		if err != nil {
			http.Error(w, "保存文件失败", http.StatusInternalServerError) // 文件保存出错时返回500状态码
			return
		}

		// 解析临时PDF文件内容（此函数需自定义实现）
		resume, err := parsePDF(tempFile.Name())
		if err != nil {
			http.Error(w, "解析PDF失败", http.StatusInternalServerError) // PDF解析出错时返回500状态码
			return
		}

		// 加载结果显示的HTML模板
		tmpl, err := template.ParseFiles("result.html")
		if err != nil {
			http.Error(w, "结果页面模板加载失败", http.StatusInternalServerError) // 模板加载失败时返回500状态码
			return
		}
		// 渲染结果模板并发送给客户端
		err = tmpl.Execute(w, resume)
		if err != nil {
			http.Error(w, "渲染结果页面失败", http.StatusInternalServerError) // 渲染结果页面出错时返回500状态码
		}
	}
}

// main 函数启动HTTP服务器
func main() {
	// 注册上传处理的路由
	http.HandleFunc("/upload", uploadHandler)

	// 配置静态文件服务，用于提供CSS、JS等静态资源
	fs := http.FileServer(http.Dir("./static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// 启动HTTP服务器并监听8080端口
	log.Println("服务器正在监听:3000")
	err := http.ListenAndServe(":3000", nil)
	if err != nil {
		log.Fatalf("服务器启动失败: %v", err) // 启动失败时打印错误信息并退出程序
	}
}
