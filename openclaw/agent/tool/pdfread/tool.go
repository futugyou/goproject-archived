package pdfread

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/futugyou/openclaw/agent/tool/pathpolicy"
	"github.com/futugyou/openclaw/core"
	"github.com/futugyou/openclaw/util"
)

type PdfReadTool struct {
	config        core.PdfReadConfig
	toolingConfig core.ToolingConfig
}

func New(
	config core.PdfReadConfig,
	toolingConfig *core.ToolingConfig) *PdfReadTool {
	if toolingConfig == nil {
		toolingConfig = &core.ToolingConfig{}
	}
	return &PdfReadTool{config: config, toolingConfig: *toolingConfig}
}

func (a *PdfReadTool) Name() string {
	return "pdf_read"
}

func (a *PdfReadTool) Description() string {
	return "Extract text content from a PDF file. Returns the text from all or specified pages."
}

func (a *PdfReadTool) ParameterSchema() string {
	return `
{
          "type": "object",
          "properties": {
            "path": {
              "type": "string",
              "description": "Path to the PDF file"
            },
            "pages": {
              "type": "string",
              "description": "Page range to extract (e.g., '1-5', '1,3,5'). Default: all pages",
              "default": "all"
            },
            "max_pages": {
              "type": "integer",
              "description": "Maximum pages to extract (0 = all)"
            }
          },
          "required": ["path"]
        } `
}

type PdfModel struct {
	Path     string `json:"path"`
	Pages    string `json:"pages"`
	MaxPages int    `json:"max_pages"`
}

func (a *PdfReadTool) Execute(ctx context.Context, argumentsJson string) string {
	if argumentsJson == "" {
		return "Error: arguments payload is empty."
	}

	var model PdfModel

	if err := json.Unmarshal([]byte(argumentsJson), &model); err != nil {
		return err.Error()
	}

	if model.Path == "" {
		return "Error: path is required."
	}
	if !util.FileExists(model.Path) {
		return fmt.Sprintf("Error: File not found: %s", model.Path)
	}
	if model.Pages == "" {
		model.Pages = "all"
	}
	if model.MaxPages < 0 {
		model.MaxPages = a.config.MaxPages
	}

	var fullPath = pathpolicy.ResolveRealPath(model.Path)

	if !pathpolicy.IsReadAllowed(a.toolingConfig, fullPath) {
		return fmt.Sprintf("Error: Read access denied for path: %s", model.Path)
	}

	externalResult, err := tryPdfToText(ctx, fullPath, model.Pages, model.MaxPages)
	if err != nil {
		return err.Error()
	}

	if externalResult != "" {
		return util.Truncate(externalResult, a.config.MaxOutputChars)
	}

	text, err := extractTextBasic(ctx, fullPath, model.MaxPages)
	if err != nil {
		return err.Error()
	}
	return util.Truncate(text, a.config.MaxOutputChars)
}

func extractTextBasic(ctx context.Context, fullPath string, maxPages int) (string, error) {
	// 读取文件字节流
	bytesData, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	// Go 默认使用 UTF-8，这里模拟 Latin1 处理原始字节串
	raw := string(bytesData)
	var sb strings.Builder

	fmt.Fprintf(&sb, "PDF: %s\n", filepath.Base(fullPath))
	sb.WriteString("Extracted via built-in parser (install pdftotext for better results)\n\n")

	pageCount := 0
	inText := false
	i := 0
	rawLen := len(raw)

	for i < rawLen-1 {
		// 检查 context 是否已被取消
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		// 粗略检查页数标记 /Page
		if i < rawLen-4 && raw[i:i+4] == "/Page" {
			pageCount++
			if maxPages > 0 && pageCount > maxPages {
				break
			}
		}

		// BT = 文本块开始
		if raw[i] == 'B' && raw[i+1] == 'T' && (i+2 >= rawLen || !isLetter(raw[i+2])) {
			inText = true
			i += 2
			continue
		}

		// ET = 文本块结束
		if raw[i] == 'E' && raw[i+1] == 'T' && (i+2 >= rawLen || !isLetter(raw[i+2])) {
			inText = false
			sb.WriteByte(' ')
			i += 2
			continue
		}

		// 提取文本块中括号内字符串
		if inText && raw[i] == '(' {
			i++
			for i < rawLen && raw[i] != ')' {
				if raw[i] == '\\' && i+1 < rawLen {
					i++ // 跳过转义符
					switch raw[i] {
					case 'n':
						sb.WriteByte('\n')
					case 'r':
						sb.WriteByte('\r')
					case 't':
						sb.WriteByte('\t')
					default:
						sb.WriteByte(raw[i])
					}
				} else {
					c := raw[i]
					if c >= 32 && c < 127 { // 可打印 ASCII 字符
						sb.WriteByte(c)
					} else if c == '\n' || c == '\r' {
						sb.WriteByte('\n')
					}
				}
				i++
			}
		}

		i++
	}

	text := strings.TrimSpace(sb.String())
	if text == "" {
		return fmt.Sprintf("PDF: %s\nNo extractable text found. "+
			"The PDF may be image-based or encrypted. Install 'pdftotext' (poppler-utils) for better extraction.",
			filepath.Base(fullPath)), nil
	}

	return text, nil
}

// 辅助函数：判断字符是否为英文字母
func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func tryPdfToText(ctx context.Context, fullPath string, pages string, maxPages int) (string, error) {
	// 1. 检查系统是否存在 (a *PdfReadTool) pdftotext 工具
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not available: %w", err)
	}

	// 2. 构建命令参数
	var args []string

	// 处理页码范围
	if pages != "all" && strings.TrimSpace(pages) != "" {
		if strings.Contains(pages, "-") {
			parts := strings.SplitN(pages, "-", 2)
			if first, err := strconv.Atoi(parts[0]); err == nil {
				args = append(args, "-f", strconv.Itoa(first))
			}
			if len(parts) > 1 {
				if last, err := strconv.Atoi(parts[1]); err == nil {
					args = append(args, "-l", strconv.Itoa(last))
				}
			}
		}
	}

	// 处理 maxPages
	if maxPages > 0 {
		args = append(args, "-l", strconv.Itoa(maxPages))
	}

	// 布局模式及输入输出参数
	args = append(args, "-layout", fullPath, "-")

	// 3. 设置超时控制 (30秒超时与上下文取消结合)
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "pdftotext", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// 4. 执行命令
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("pdftotext execution failed: %w", err)
	}

	output := stdout.String()
	header := fmt.Sprintf("PDF: %s\nExtracted via pdftotext\nLength: %d chars\n\n",
		filepath.Base(fullPath), len(output))

	return header + output, nil
}
