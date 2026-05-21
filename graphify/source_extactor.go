package graphify

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var _ IPipelineStage[DetectedFile, ExtractionResult] = (*SourceExtractor)(nil)

type SourceExtractor struct {
	extractors map[string]ILanguageExtractor
}

func NewSourceExtractor() *SourceExtractor {
	return &SourceExtractor{
		extractors: map[string]ILanguageExtractor{
			"csharp":     NewCsharpExtractor(),
			"python":     NewPythonExtractor(),
			"javaScript": NewJavaScriptExtractor(),
			"typeScript": NewTypeScriptExtractor(),
			"go":         NewGoExtractor(),
			"java":       NewJavaExtractor(),
			"rust":       NewRustExtractor(),
			"c":          NewCExtractor(),
			"cpp":        NewCppExtractor(),
		},
	}
}

// Execute implements [IPipelineStage].
func (e *SourceExtractor) Execute(ctx context.Context, input DetectedFile) (*ExtractionResult, error) {
	panic("unimplemented")
}

type LanguageExtractorModel struct {
	Nodes []ExtractedNode
	Edges []ExtractedEdge
}

type ILanguageExtractor interface {
	Extract(content, filePath, fileName string) (*LanguageExtractorModel, error)
}

func LanguageMakeId(parts []string) string {
	combined := strings.Join(parts, "_")
	reScript := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	cleaned := reScript.ReplaceAllString(combined, "_")

	return strings.ToLower(strings.Trim(cleaned, "_"))
}

func LanguageCreateNode(id, label, filePath string, line int) ExtractedNode {
	return ExtractedNode{
		Id:             id,
		Label:          label,
		FileType:       FileTypeCode,
		SourceFile:     filePath,
		SourceLocation: fmt.Sprintf("L%d", line),
	}
}

func LanguageCreateEdge(source, target, relation, filePath string, line int, confidence Confidence, weight *float32) ExtractedEdge {
	var w float32 = 1.0
	if weight != nil {
		w = *weight
	}

	if confidence == ConfidenceUnkown {
		confidence = ConfidenceExtracted
	}

	return ExtractedEdge{
		Source:         source,
		Target:         target,
		Relation:       relation,
		Confidence:     confidence,
		SourceFile:     filePath,
		SourceLocation: fmt.Sprintf("L%d", line),
		Weight:         w,
	}
}

func LanguageGetLineNumber(content string, index int) int {
	if index <= 0 {
		return 1
	}
	return strings.Count(content[:index], "\n") + 1
}

type csharpExtractor struct {
	NamespacePattern *regexp.Regexp
	UsingPattern     *regexp.Regexp
	ClassPattern     *regexp.Regexp
	MethodPattern    *regexp.Regexp
}

func NewCsharpExtractor() *csharpExtractor {
	return &csharpExtractor{
		NamespacePattern: regexp.MustCompile(`(?m)^\s*namespace\s+([a-zA-Z_][\w.]*)`),
		UsingPattern:     regexp.MustCompile(`(?m)^\s*using\s+([a-zA-Z_][\w.]*)\s*;`),
		ClassPattern:     regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal|static|abstract|sealed|partial)*\s*(?:class|interface|struct|record)\s+([a-zA-Z_]\w+)`),
		MethodPattern:    regexp.MustCompile(`(?m)^\s*(?:public|private|protected|internal|static|virtual|override|async|abstract)*\s+[\w<>[\], \t]+\s+([a-zA-Z_]\w+)\s*\(`),
	}
}

func (c *csharpExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	var fileId = LanguageMakeId([]string{stem})
	var nodes = []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	var edges = []ExtractedEdge{}

	// Extract using directives
	for _, match := range c.UsingPattern.FindAllStringSubmatchIndex(content, -1) {
		group1Start := match[2]
		group1End := match[3]
		fullModule := content[group1Start:group1End]
		parts := strings.Split(fullModule, ".")
		module := parts[len(parts)-1]

		targetId := LanguageMakeId([]string{module})
		entireMatchStart := match[0]
		line := LanguageGetLineNumber(content, entireMatchStart)

		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract namespaces
	for _, match := range c.NamespacePattern.FindAllStringSubmatchIndex(content, -1) {
		group1Start := match[2]
		group1End := match[3]
		nsName := content[group1Start:group1End]
		parts := strings.Split(nsName, ".")
		module := parts[len(parts)-1]

		nsId := LanguageMakeId([]string{module})
		entireMatchStart := match[0]
		line := LanguageGetLineNumber(content, entireMatchStart)
		nodes = append(nodes, LanguageCreateNode(nsId, nsName, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, nsId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract classes/interfaces/structs
	for _, match := range c.ClassPattern.FindAllStringSubmatchIndex(content, -1) {
		group1Start := match[2]
		group1End := match[3]
		className := content[group1Start:group1End]
		parts := strings.Split(className, ".")
		module := parts[len(parts)-1]

		classId := LanguageMakeId([]string{module})
		entireMatchStart := match[0]
		line := LanguageGetLineNumber(content, entireMatchStart)
		nodes = append(nodes, LanguageCreateNode(classId, className, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, classId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract methods
	for _, match := range c.MethodPattern.FindAllStringSubmatchIndex(content, -1) {
		group1Start := match[2]
		group1End := match[3]
		methodName := content[group1Start:group1End]
		parts := strings.Split(methodName, ".")
		module := parts[len(parts)-1]

		methodId := LanguageMakeId([]string{module})
		entireMatchStart := match[0]
		line := LanguageGetLineNumber(content, entireMatchStart)
		nodes = append(nodes, LanguageCreateNode(methodId, methodName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, methodId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{
		Nodes: nodes,
		Edges: edges,
	}, nil
}

// ==========================================
// Python Extractor
// ==========================================

type pythonExtractor struct {
	ImportStringPattern *regexp.Regexp
	FromImportPattern   *regexp.Regexp
	ClassPattern        *regexp.Regexp
	FunctionPattern     *regexp.Regexp
}

func NewPythonExtractor() *pythonExtractor {
	return &pythonExtractor{
		ImportStringPattern: regexp.MustCompile(`(?m)^\s*import\s+([a-zA-Z_][\w.]*)`),
		FromImportPattern:   regexp.MustCompile(`(?m)^\s*from\s+([a-zA-Z_][\w.]*)\s+import`),
		ClassPattern:        regexp.MustCompile(`(?m)^\s*class\s+([a-zA-Z_]\w+)`),
		FunctionPattern:     regexp.MustCompile(`(?m)^\s*def\s+([a-zA-Z_]\w+)\s*\(`),
	}
}

func (p *pythonExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})
	nodes := []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	edges := []ExtractedEdge{}

	// Extract imports
	for _, match := range p.ImportStringPattern.FindAllStringSubmatchIndex(content, -1) {
		fullModule := content[match[2]:match[3]]
		parts := strings.Split(fullModule, ".")
		module := parts[len(parts)-1]

		targetId := LanguageMakeId([]string{module})
		line := LanguageGetLineNumber(content, match[0])
		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract from ... import
	for _, match := range p.FromImportPattern.FindAllStringSubmatchIndex(content, -1) {
		fullModule := content[match[2]:match[3]]
		parts := strings.Split(fullModule, ".")
		module := parts[len(parts)-1]

		targetId := LanguageMakeId([]string{module})
		line := LanguageGetLineNumber(content, match[0])
		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports_from", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract classes
	for _, match := range p.ClassPattern.FindAllStringSubmatchIndex(content, -1) {
		className := content[match[2]:match[3]]
		classId := LanguageMakeId([]string{stem, className})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(classId, className, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, classId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract functions
	for _, match := range p.FunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		funcName := content[match[2]:match[3]]
		funcId := LanguageMakeId([]string{stem, funcName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(funcId, funcName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, funcId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{Nodes: nodes, Edges: edges}, nil
}

// ==========================================
// JavaScript Extractor
// ==========================================

type javaScriptExtractor struct {
	ImportPattern        *regexp.Regexp
	ClassPattern         *regexp.Regexp
	FunctionPattern      *regexp.Regexp
	ArrowFunctionPattern *regexp.Regexp
}

func NewJavaScriptExtractor() *javaScriptExtractor {
	return &javaScriptExtractor{
		ImportPattern:        regexp.MustCompile(`(?m)^\s*import\s+.*?\s+from\s+["'"]([^"'"]+)["'"]`),
		ClassPattern:         regexp.MustCompile(`(?m)^\s*(?:export\s+)?class\s+([a-zA-Z_]\w+)`),
		FunctionPattern:      regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+([a-zA-Z_]\w+)\s*\(`),
		ArrowFunctionPattern: regexp.MustCompile(`(?m)^\s*(?:const|let|var)\s+([a-zA-Z_]\w+)\s*=\s*(?:async\s+)?\(`),
	}
}

func (js *javaScriptExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})
	nodes := []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	edges := []ExtractedEdge{}

	// Extract imports
	for _, match := range js.ImportPattern.FindAllStringSubmatchIndex(content, -1) {
		importPath := content[match[2]:match[3]]
		// Split by '/' and take last
		pathParts := strings.Split(importPath, "/")
		lastPart := pathParts[len(pathParts)-1]
		lastPart = strings.TrimLeft(lastPart, ".")
		// Split by '.' and take first
		module := strings.Split(lastPart, ".")[0]

		if module != "" {
			targetId := LanguageMakeId([]string{module})
			line := LanguageGetLineNumber(content, match[0])
			edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports_from", filePath, line, ConfidenceUnkown, nil))
		}
	}

	// Extract classes
	for _, match := range js.ClassPattern.FindAllStringSubmatchIndex(content, -1) {
		className := content[match[2]:match[3]]
		classId := LanguageMakeId([]string{stem, className})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(classId, className, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, classId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract functions
	for _, match := range js.FunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		funcName := content[match[2]:match[3]]
		funcId := LanguageMakeId([]string{stem, funcName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(funcId, funcName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, funcId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract arrow functions
	for _, match := range js.ArrowFunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		funcName := content[match[2]:match[3]]
		funcId := LanguageMakeId([]string{stem, funcName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(funcId, funcName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, funcId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{Nodes: nodes, Edges: edges}, nil
}

// ==========================================
// TypeScript Extractor
// ==========================================

type typeScriptExtractor struct {
	javaScriptExtractor
	InterfacePattern *regexp.Regexp
}

func NewTypeScriptExtractor() *typeScriptExtractor {
	jsExt := NewJavaScriptExtractor()
	return &typeScriptExtractor{
		javaScriptExtractor: *jsExt,
		InterfacePattern:    regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+([a-zA-Z_]\w+)`),
	}
}

func (ts *typeScriptExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	model, err := ts.javaScriptExtractor.Extract(content, filePath, fileName)
	if err != nil {
		return nil, err
	}

	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})

	// Extract interfaces
	for _, match := range ts.InterfacePattern.FindAllStringSubmatchIndex(content, -1) {
		interfaceName := content[match[2]:match[3]]
		interfaceId := LanguageMakeId([]string{stem, interfaceName})
		line := LanguageGetLineNumber(content, match[0])

		model.Nodes = append(model.Nodes, LanguageCreateNode(interfaceId, interfaceName, filePath, line))
		model.Edges = append(model.Edges, LanguageCreateEdge(fileId, interfaceId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return model, nil
}

// ==========================================
// Go Extractor
// ==========================================

type goExtractor struct {
	ImportPattern   *regexp.Regexp
	TypePattern     *regexp.Regexp
	FunctionPattern *regexp.Regexp
}

func NewGoExtractor() *goExtractor {
	return &goExtractor{
		ImportPattern:   regexp.MustCompile(`(?m)^\s*import\s+["'"]([^"'"]+)["'"]`),
		TypePattern:     regexp.MustCompile(`(?m)^\s*type\s+([a-zA-Z_]\w+)\s+(?:struct|interface)`),
		FunctionPattern: regexp.MustCompile(`(?m)^\s*func\s+(?:\([^)]*\)\s+)?([a-zA-Z_]\w+)\s*\(`),
	}
}

func (g *goExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})
	nodes := []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	edges := []ExtractedEdge{}

	// Extract imports
	for _, match := range g.ImportPattern.FindAllStringSubmatchIndex(content, -1) {
		importPath := content[match[2]:match[3]]
		parts := strings.Split(importPath, "/")
		module := parts[len(parts)-1]

		targetId := LanguageMakeId([]string{module})
		line := LanguageGetLineNumber(content, match[0])
		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract types
	for _, match := range g.TypePattern.FindAllStringSubmatchIndex(content, -1) {
		typeName := content[match[2]:match[3]]
		typeId := LanguageMakeId([]string{stem, typeName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(typeId, typeName, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, typeId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract functions
	for _, match := range g.FunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		funcName := content[match[2]:match[3]]
		funcId := LanguageMakeId([]string{stem, funcName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(funcId, funcName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, funcId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{Nodes: nodes, Edges: edges}, nil
}

// ==========================================
// Java Extractor
// ==========================================

type javaExtractor struct {
	ImportPattern *regexp.Regexp
	ClassPattern  *regexp.Regexp
	MethodPattern *regexp.Regexp
}

func NewJavaExtractor() *javaExtractor {
	return &javaExtractor{
		ImportPattern: regexp.MustCompile(`(?m)^\s*import\s+([a-zA-Z_][\w.]*)\s*;`),
		ClassPattern:  regexp.MustCompile(`(?m)^\s*(?:public|private|protected|abstract|final)*\s*(?:class|interface|enum)\s+([a-zA-Z_]\w+)`),
		MethodPattern: regexp.MustCompile(`(?m)^\s*(?:public|private|protected|static|final|abstract|synchronized)*\s+[\w<>[\],\s]+\s+([a-zA-Z_]\w+)\s*\(`),
	}
}

func (j *javaExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})
	nodes := []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	edges := []ExtractedEdge{}

	// Extract imports
	for _, match := range j.ImportPattern.FindAllStringSubmatchIndex(content, -1) {
		importPath := content[match[2]:match[3]]
		parts := strings.Split(importPath, ".")
		module := parts[len(parts)-1]

		targetId := LanguageMakeId([]string{module})
		line := LanguageGetLineNumber(content, match[0])
		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract classes
	for _, match := range j.ClassPattern.FindAllStringSubmatchIndex(content, -1) {
		className := content[match[2]:match[3]]
		classId := LanguageMakeId([]string{stem, className})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(classId, className, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, classId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract methods
	for _, match := range j.MethodPattern.FindAllStringSubmatchIndex(content, -1) {
		methodName := content[match[2]:match[3]]
		methodId := LanguageMakeId([]string{stem, methodName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(methodId, methodName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, methodId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{Nodes: nodes, Edges: edges}, nil
}

// ==========================================
// Rust Extractor
// ==========================================

type rustExtractor struct {
	UsePattern      *regexp.Regexp
	StructPattern   *regexp.Regexp
	TraitPattern    *regexp.Regexp
	FunctionPattern *regexp.Regexp
}

func NewRustExtractor() *rustExtractor {
	return &rustExtractor{
		UsePattern:      regexp.MustCompile(`(?m)^\s*use\s+([a-zA-Z_][\w:]*)`),
		StructPattern:   regexp.MustCompile(`(?m)^\s*(?:pub\s+)?struct\s+([a-zA-Z_]\w+)`),
		TraitPattern:    regexp.MustCompile(`(?m)^\s*(?:pub\s+)?trait\s+([a-zA-Z_]\w+)`),
		FunctionPattern: regexp.MustCompile(`(?m)^\s*(?:pub\s+)?(?:async\s+)?fn\s+([a-zA-Z_]\w+)\s*[<(]`),
	}
}

func (r *rustExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})
	nodes := []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	edges := []ExtractedEdge{}

	// Extract uses
	for _, match := range r.UsePattern.FindAllStringSubmatchIndex(content, -1) {
		usePath := content[match[2]:match[3]]
		parts := strings.Split(usePath, "::")
		lastPart := parts[len(parts)-1]
		module := strings.NewReplacer("{", "", "}", "", " ", "").Replace(lastPart)

		targetId := LanguageMakeId([]string{module})
		line := LanguageGetLineNumber(content, match[0])
		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract structs
	for _, match := range r.StructPattern.FindAllStringSubmatchIndex(content, -1) {
		structName := content[match[2]:match[3]]
		structId := LanguageMakeId([]string{stem, structName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(structId, structName, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, structId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract traits
	for _, match := range r.TraitPattern.FindAllStringSubmatchIndex(content, -1) {
		traitName := content[match[2]:match[3]]
		traitId := LanguageMakeId([]string{stem, traitName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(traitId, traitName, filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, traitId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract functions
	for _, match := range r.FunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		funcName := content[match[2]:match[3]]
		funcId := LanguageMakeId([]string{stem, funcName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(funcId, funcName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, funcId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{Nodes: nodes, Edges: edges}, nil
}

// ==========================================
// C Extractor
// ==========================================

type cExtractor struct {
	IncludePattern  *regexp.Regexp
	FunctionPattern *regexp.Regexp
}

func NewCExtractor() *cExtractor {
	return &cExtractor{
		IncludePattern:  regexp.MustCompile(`(?m)^\s*#include\s+[<"]([^>"]+)[>"]`),
		FunctionPattern: regexp.MustCompile(`(?m)^\s*(?:static\s+)?(?:inline\s+)?(?:extern\s+)?[\w\s*]+\s+([a-zA-Z_]\w+)\s*\([^)]*\)\s*\{`),
	}
}

func (c *cExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})
	nodes := []ExtractedNode{LanguageCreateNode(fileId, fileName, filePath, 1)}
	edges := []ExtractedEdge{}

	// Extract includes
	for _, match := range c.IncludePattern.FindAllStringSubmatchIndex(content, -1) {
		includePath := content[match[2]:match[3]]
		pathParts := strings.Split(includePath, "/")
		lastPart := pathParts[len(pathParts)-1]
		module := strings.TrimSuffix(lastPart, filepath.Ext(lastPart))

		targetId := LanguageMakeId([]string{module})
		line := LanguageGetLineNumber(content, match[0])
		edges = append(edges, LanguageCreateEdge(fileId, targetId, "imports", filePath, line, ConfidenceUnkown, nil))
	}

	// Extract functions
	for _, match := range c.FunctionPattern.FindAllStringSubmatchIndex(content, -1) {
		funcName := content[match[2]:match[3]]

		// Skip common keywords that might be matched
		if funcName == "if" || funcName == "while" || funcName == "for" || funcName == "switch" || funcName == "return" {
			continue
		}

		funcId := LanguageMakeId([]string{stem, funcName})
		line := LanguageGetLineNumber(content, match[0])

		nodes = append(nodes, LanguageCreateNode(funcId, funcName+"()", filePath, line))
		edges = append(edges, LanguageCreateEdge(fileId, funcId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return &LanguageExtractorModel{Nodes: nodes, Edges: edges}, nil
}

// ==========================================
// C++ Extractor
// ==========================================

type cppExtractor struct {
	cExtractor
	ClassPattern *regexp.Regexp
}

func NewCppExtractor() *cppExtractor {
	cExt := NewCExtractor()
	return &cppExtractor{
		cExtractor:   *cExt,
		ClassPattern: regexp.MustCompile(`(?m)^\s*(?:class|struct)\s+([a-zA-Z_]\w+)`),
	}
}

func (cpp *cppExtractor) Extract(content, filePath, fileName string) (*LanguageExtractorModel, error) {
	model, err := cpp.cExtractor.Extract(content, filePath, fileName)
	if err != nil {
		return nil, err
	}

	stem := strings.TrimSuffix(filepath.Base(fileName), filepath.Ext(fileName))
	fileId := LanguageMakeId([]string{stem})

	// Extract classes
	for _, match := range cpp.ClassPattern.FindAllStringSubmatchIndex(content, -1) {
		className := content[match[2]:match[3]]
		classId := LanguageMakeId([]string{stem, className})
		line := LanguageGetLineNumber(content, match[0])

		model.Nodes = append(model.Nodes, LanguageCreateNode(classId, className, filePath, line))
		model.Edges = append(model.Edges, LanguageCreateEdge(fileId, classId, "contains", filePath, line, ConfidenceUnkown, nil))
	}

	return model, nil
}
