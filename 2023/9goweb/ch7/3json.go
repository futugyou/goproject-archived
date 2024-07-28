package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Post struct {
	Id       int       `json:"id"`
	Content  string    `json:"content"`
	Author   Author    `json:"author"`
	Comments []Comment `json:"comments"`
}

type Author struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
}

type Comment struct {
	Id      int    `json:"id"`
	Content string `json:"content"`
	Author  string `json:"author"`
}

func main() {
	post := Post{
		Id:      1,
		Content: "Hello",
		Author: Author{
			Id:   2,
			Name: "same",
		},
		Comments: []Comment{
			Comment{
				Id:      3,
				Content: "A",
				Author:  "AA",
			},
			Comment{
				Id:      4,
				Content: "B",
				Author:  "BB",
			},
		},
	}

	//1
	output, err := json.MarshalIndent(&post, "", "\t\t")
	if err != nil {
		fmt.Println(err)
		return
	}

	err = ioutil.WriteFile("post.json", output, 0644)
	if err != nil {
		fmt.Println(err)
	}

	//2
	jsonFile, err := os.Create("post1.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	encoder := json.NewEncoder(jsonFile)
	err = encoder.Encode(&post)
	if err != nil {
		fmt.Println(err)
		return
	}
}
