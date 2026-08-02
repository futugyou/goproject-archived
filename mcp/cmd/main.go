package main

import (
	"encoding/json"
	"fmt"

	"github.com/futugyou/mcp/core"
	"github.com/google/jsonschema-go/jsonschema"
)

func main() {
	a, err := jsonschema.For[core.StringSchema](nil)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	expectedJSON, err := json.Marshal(a)
	if err != nil {

		fmt.Println(err.Error())
		return
	}

	fmt.Println(string(expectedJSON))
}
