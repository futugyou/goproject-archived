package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Human struct {
	name   string `json:"name"`
	Gender string `json:"s"`
	Ang    int    `json:"Age"`
	Lesson
}

type Lesson struct {
	Lessons []string `json:"lessons"`
}

func main() {
	jsonStr := `{"Age": 18,"name": "Jim" ,"s": "男",
	"lessons":["English","History"],"Room":201,"n":null,"b":false}`
	strR := strings.NewReader(jsonStr)
	h := &Human{}

	err := json.NewDecoder(strR)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(h)

	f, err := os.Create("./t.json")
	json.NewEncoder(f).Encode(h)
}
