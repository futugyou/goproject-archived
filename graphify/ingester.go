package graphify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type UrlType int

const (
	Webpage UrlType = iota
	ArxivPaper
	GitHubRepo
)

type UrlIngester struct {
	httpClient *http.Client
}

func NewUrlIngester(client *http.Client) *UrlIngester {
	if client == nil {
		client = &http.Client{
			Timeout: 30 * time.Second,
		}
	}
	return &UrlIngester{
		httpClient: client,
	}
}

func (u *UrlIngester) IngestAsync(ctx context.Context, rawUrl string) (string, error) {
	if strings.TrimSpace(rawUrl) == "" {
		return "", fmt.Errorf("url cannot be empty")
	}

	if err := u.validateUrl(rawUrl); err != nil {
		return "", err
	}

	urlType := u.detectUrlType(rawUrl)

	switch urlType {
	case ArxivPaper:
		return u.fetchArxivPaperAsync(ctx, rawUrl)
	case GitHubRepo:
		return u.fetchGitHubRepoAsync(ctx, rawUrl)
	default:
		return u.fetchWebpageAsync(ctx, rawUrl)
	}
}

func (u *UrlIngester) IngestToFileAsync(ctx context.Context, rawUrl, outputDir, author string) (string, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", err
	}

	content, err := u.IngestAsync(ctx, rawUrl)
	if err != nil {
		return "", err
	}

	if strings.TrimSpace(author) != "" {
		content = u.addContributorMetadata(content, author)
	}

	filename := u.generateSafeFilename(rawUrl) + ".md"
	outputPath := filepath.Join(outputDir, filename)

	counter := 1
	for {
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			break
		}
		stem := strings.TrimSuffix(filename, ".md")
		outputPath = filepath.Join(outputDir, fmt.Sprintf("%s_%d.md", stem, counter))
		counter++
	}

	err = os.WriteFile(outputPath, []byte(content), 0644)
	return outputPath, err
}

func (u *UrlIngester) fetchWebpageAsync(ctx context.Context, rawUrl string) (string, error) {
	html, err := u.doGet(ctx, rawUrl)
	if err != nil {
		return "", err
	}

	title := u.extractTitle(html)
	if title == "" {
		title = rawUrl
	}

	markdown := u.htmlToMarkdown(html)

	contentLimit := min(len(markdown), 12000)
	truncated := markdown[:contentLimit]

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "source_url: %s\n", rawUrl)
	sb.WriteString("type: webpage\n")
	fmt.Fprintf(&sb, "title: \"%s\"\n", u.escapeYaml(title))
	fmt.Fprintf(&sb, "captured_at: %s\n", time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("---\n\n")
	fmt.Fprintf(&sb, "# %s\n\n", title)
	fmt.Fprintf(&sb, "Source: %s\n\n---\n\n", rawUrl)
	sb.WriteString(truncated)

	if len(markdown) > 12000 {
		sb.WriteString("\n\n*[Content truncated]*")
	}

	return sb.String(), nil
}

func (u *UrlIngester) fetchArxivPaperAsync(ctx context.Context, rawUrl string) (string, error) {
	re := regexp.MustCompile(`(\d{4}\.\d{4,5})`)
	match := re.FindStringSubmatch(rawUrl)
	if match == nil {
		return u.fetchWebpageAsync(ctx, rawUrl)
	}

	arxivId := match[1]
	absUrl := fmt.Sprintf("https://export.arxiv.org/abs/%s", arxivId)

	html, err := u.doGet(ctx, absUrl)
	if err != nil {
		return u.fetchWebpageAsync(ctx, rawUrl)
	}

	title := u.extractArxivTitle(html)
	authors := u.extractArxivAuthors(html)
	abstract := u.extractArxivAbstract(html)

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "source_url: %s\n", rawUrl)
	fmt.Fprintf(&sb, "arxiv_id: %s\n", arxivId)
	sb.WriteString("type: paper\n")
	fmt.Fprintf(&sb, "title: \"%s\"\n", u.escapeYaml(title))
	if authors != "" {
		fmt.Fprintf(&sb, "paper_authors: \"%s\"\n", u.escapeYaml(authors))
	}
	fmt.Fprintf(&sb, "captured_at: %s\n", time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("---\n\n")
	fmt.Fprintf(&sb, "# %s\n\n", title)
	if authors != "" {
		fmt.Fprintf(&sb, "**Authors:** %s\n", authors)
	}
	fmt.Fprintf(&sb, "**arXiv:** %s\n\n", arxivId)
	sb.WriteString("## Abstract\n\n")
	sb.WriteString(abstract)
	sb.WriteString("\n\n")
	fmt.Fprintf(&sb, "Source: %s", rawUrl)

	return sb.String(), nil
}

func (u *UrlIngester) fetchGitHubRepoAsync(ctx context.Context, rawUrl string) (string, error) {
	re := regexp.MustCompile(`github\.com/([^/]+)/([^/]+)`)
	match := re.FindStringSubmatch(rawUrl)
	if match == nil {
		return u.fetchWebpageAsync(ctx, rawUrl)
	}

	owner := match[1]
	repo := strings.TrimSuffix(match[2], "/")

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "source_url: %s\n", rawUrl)
	sb.WriteString("type: github_repo\n")
	fmt.Fprintf(&sb, "github_owner: %s\n", owner)
	fmt.Fprintf(&sb, "github_repo: %s\n", repo)
	fmt.Fprintf(&sb, "captured_at: %s\n", time.Now().UTC().Format(time.RFC3339))
	sb.WriteString("---\n\n")
	fmt.Fprintf(&sb, "# %s/%s\n\n", owner, repo)
	fmt.Fprintf(&sb, "GitHub Repository: %s\n\n", rawUrl)
	sb.WriteString("*Use GitHub API or clone the repository for full code analysis.*")

	return sb.String(), nil
}

func (u *UrlIngester) doGet(ctx context.Context, targetUrl string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", targetUrl, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "graphify-go/1.0")

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	return string(body), err
}

func (u *UrlIngester) detectUrlType(rawUrl string) UrlType {
	lower := strings.ToLower(rawUrl)
	if strings.Contains(lower, "arxiv.org") {
		return ArxivPaper
	}
	if strings.Contains(lower, "github.com") {
		return GitHubRepo
	}
	return Webpage
}

func (u *UrlIngester) htmlToMarkdown(html string) string {
	content := html
	reScript := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	content = reScript.ReplaceAllString(content, "")
	reStyle := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	content = reStyle.ReplaceAllString(content, "")

	reArticle := regexp.MustCompile(`(?is)<article[^>]*>(.*?)</article>`)
	if match := reArticle.FindStringSubmatch(content); match != nil {
		content = match[1]
	} else {
		reMain := regexp.MustCompile(`(?is)<main[^>]*>(.*?)</main>`)
		if match := reMain.FindStringSubmatch(content); match != nil {
			content = match[1]
		}
	}

	reH1 := regexp.MustCompile(`(?is)<h1[^>]*>(.*?)</h1>`)
	content = reH1.ReplaceAllString(content, "\n# $1\n")
	reP := regexp.MustCompile(`(?is)<p[^>]*>(.*?)</p>`)
	content = reP.ReplaceAllString(content, "$1\n\n")

	reTags := regexp.MustCompile(`<[^>]+>`)
	content = reTags.ReplaceAllString(content, " ")

	content = regexp.MustCompile(`[ \t]+`).ReplaceAllString(content, " ")
	content = regexp.MustCompile(`\n\n\n+`).ReplaceAllString(content, "\n\n")

	return strings.TrimSpace(content)
}

func (u *UrlIngester) extractTitle(html string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	match := re.FindStringSubmatch(html)
	if match != nil {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func (u *UrlIngester) extractArxivTitle(html string) string {
	re := regexp.MustCompile(`(?is)class="title[^"]*"[^>]*>(.*?)</h1>`)
	match := re.FindStringSubmatch(html)
	if match != nil {
		title := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(match[1], "")
		return strings.TrimSpace(title)
	}
	return ""
}

func (u *UrlIngester) extractArxivAuthors(html string) string {
	re := regexp.MustCompile(`(?is)class="authors"[^>]*>(.*?)</div>`)
	match := re.FindStringSubmatch(html)
	if match != nil {
		authors := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(match[1], "")
		return strings.TrimSpace(authors)
	}
	return ""
}

func (u *UrlIngester) extractArxivAbstract(html string) string {
	re := regexp.MustCompile(`(?is)class="abstract[^"]*"[^>]*>(.*?)</blockquote>`)
	match := re.FindStringSubmatch(html)
	if match != nil {
		abs := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(match[1], "")
		return strings.TrimSpace(abs)
	}
	return ""
}

func (u *UrlIngester) generateSafeFilename(rawUrl string) string {
	parsed, _ := url.Parse(rawUrl)
	name := parsed.Host + parsed.Path
	re := regexp.MustCompile(`[^\w\-]`)
	name = re.ReplaceAllString(name, "_")
	name = regexp.MustCompile(`_+`).ReplaceAllString(name, "_")
	name = strings.Trim(name, "_")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

func (u *UrlIngester) escapeYaml(val string) string {
	val = strings.ReplaceAll(val, "\\", "\\\\")
	val = strings.ReplaceAll(val, "\"", "\\\"")
	val = strings.ReplaceAll(val, "\n", "\\n")
	return val
}

func (u *UrlIngester) addContributorMetadata(content, author string) string {
	lines := strings.Split(content, "\n")
	var result strings.Builder
	inFrontmatter := false
	added := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				result.WriteString(line)
				result.WriteString("\n")
			} else {
				if !added {
					fmt.Fprintf(&result, "contributor: \"%s\"\n", u.escapeYaml(author))
					added = true
				}
				result.WriteString(line)
				result.WriteString("\n")
				inFrontmatter = false
			}
		} else {
			result.WriteString(line)
			result.WriteString("\n")
		}
	}
	return result.String()
}

func (u *UrlIngester) validateUrl(rawUrl string) error {
	_, err := url.ParseRequestURI(rawUrl)
	return err
}
