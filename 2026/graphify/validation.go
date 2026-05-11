package graphify

import (
	"fmt"
)

type ValidationResult struct {
	IsValid bool
	Errors  []string
}

func ValidationSuccess() ValidationResult {
	return ValidationResult{IsValid: true}
}

func ValidationFailure(err []string) ValidationResult {
	return ValidationResult{Errors: err, IsValid: false}
}

type ExtractionValidator struct {
}

func (e *ExtractionValidator) Validate(extraction ExtractionResult) ValidationResult {
	errors := []string{}

	errors = e.validateNodes(extraction.Nodes, errors)
	errors = e.validateEdges(extraction.Edges, extraction.Nodes, errors)

	if len(errors) == 0 {
		return ValidationSuccess()
	} else {
		return ValidationFailure(errors)
	}
}

func (e *ExtractionValidator) validateNodes(nodes []ExtractedNode, errs []string) []string {
	if len(nodes) == 0 {
		return append(errs, "Nodes list cannot be null")
	}

	t := []string{}
	for i := range nodes {
		if len(nodes[i].Id) == 0 {
			t = append(t, fmt.Sprintf("Node %d has empty or null Id", i))
		}
		if len(nodes[i].Label) == 0 {
			t = append(t, fmt.Sprintf("Node %d has empty or null Label", i))
		}
		if len(nodes[i].SourceFile) == 0 {
			t = append(t, fmt.Sprintf("Node %d has empty or null SourceFile", i))
		}
	}

	return append(errs, t...)
}

func (e *ExtractionValidator) validateEdges(edges []ExtractedEdge, nodes []ExtractedNode, errs []string) []string {
	if len(edges) == 0 {
		return append(errs, "Edges list cannot be null")
	}

	nodeIds := map[string]struct{}{}
	for _, v := range nodes {
		nodeIds[v.Id] = struct{}{}
	}
	t := []string{}
	for i := range edges {

		if len(edges[i].Source) == 0 {
			t = append(t, fmt.Sprintf("Edge %d has empty or null Source", i))
		} else if _, ok := nodeIds[edges[i].Source]; len(nodeIds) > 0 && !ok {
			t = append(t, fmt.Sprintf("Edge %d source '%s' does not match any node id", i, edges[i].Source))

		}
		if len(edges[i].Target) == 0 {
			t = append(t, fmt.Sprintf("Edge %d has empty or null Target", i))
		} else if _, ok := nodeIds[edges[i].Target]; len(nodeIds) > 0 && !ok {
			t = append(t, fmt.Sprintf("Edge %d Target '%s' does not match any node id", i, edges[i].Target))

		}
		if len(edges[i].Relation) == 0 {
			t = append(t, fmt.Sprintf("Edge %d has empty or null Relation", i))
		}
		if len(edges[i].SourceFile) == 0 {
			t = append(t, fmt.Sprintf("Edge %d has empty or null SourceFile", i))
		}
	}

	return append(errs, t...)
}
