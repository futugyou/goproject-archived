package main

import (
	"fmt"

	first "github.com/solenovex/protobuf-go/first"
)

func main() {
	NewPersonMessage()
}

func NewPersonMessage() {
	pm := first.PersonMessage{
		Id:           1234,
		Is_Adult:     true,
		Name:         "Dave",
		LuckyNumbers: []int32{1, 2, 3, 5},
	}

	fmt.Println(pm)
	pm.Name = "Nick"
	fmt.Println(pm)
	fmt.Printf("the Id is %d", pm.GetId())

}

//protoc --proto_path ./ --go_out=./ ./first/person.proto
