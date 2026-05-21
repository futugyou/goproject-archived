package graphify

import (
	"html/template"
	"os"
)

var CommunityColors []string = []string{
	"#4E79A7", "#F28E2B", "#E15759", "#76B7B2", "#59A14F",
	"#EDC948", "#B07AA1", "#FF9DA7", "#9C755F", "#BAB0AC",
}

const MaxNodesForVisualization int = 5000

type HtmlGraphData struct {
	Title      string
	NodesJSON  template.JS
	EdgesJSON  template.JS
	LegendJSON template.JS
	Stats      string
}

func HtmlTemplateGenerate(title, nodesJson, edgesJson, legendJson, stats string, f *os.File) error {
	tmpl, err := template.New("graph").ParseFiles("graph.html.tmpl")
	if err != nil {
		return err
	}

	data := HtmlGraphData{
		Title:      title,
		NodesJSON:  template.JS(nodesJson),
		EdgesJSON:  template.JS(edgesJson),
		LegendJSON: template.JS(legendJson),
		Stats:      stats,
	}

	return tmpl.Execute(f, data)
}
