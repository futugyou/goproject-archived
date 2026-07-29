package protocol

import "fmt"

type Reference struct {
	Name  string `json:"name"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Url   string `json:"url"`
}

func NewResourceTemplateReference(url string) *Reference {
	return &Reference{
		Type: "ref/resource",
		Url:  url,
	}
}

func NewPromptReference(name string, title string) *Reference {
	return &Reference{
		Type:  "ref/prompt",
		Name:  name,
		Title: title,
	}
}

func (r *Reference) IsResourceTemplateReference() bool {
	return r.Type == "ref/resource"
}

func (r *Reference) IsPromptReference() bool {
	return r.Type == "ref/prompt"
}

func (r *Reference) ToString() string {
	switch r.Type {
	case "ref/resource":
		return fmt.Sprintf("\"%s\": \"%s\"", r.Type, r.Url)
	case "ref/prompt":
		return fmt.Sprintf("\"%s\": \"%s\"", r.Type, r.Name)
	default:
		return ""
	}

}

func (r *Reference) Validate() (string, bool) {
	validationMessage := ""
	switch r.Type {
	case "ref/resource":
		if len(r.Url) == 0 {
			validationMessage = "Uri is required for ref/resource"
			return validationMessage, false
		}
	case "ref/prompt":
		if len(r.Name) == 0 {
			validationMessage = "Name is required for ref/prompt"
			return validationMessage, false
		}
	default:
		validationMessage = "Unknown reference type: " + r.Type
		return validationMessage, false
	}

	return validationMessage, true
}
