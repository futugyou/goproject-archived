package main

import (
	context "context"
	"log"
	sub "work/golang-test/advancedgolang/ch4/4-4/pubsubservice"
	fmt "fmt"
	"io"
	grpc "google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:1234", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := sub.NewPubsubServiceClient(conn)

	stream, err := client.Subscribe(context.Background(), &sub.String{Value: "golang:"})
	if err!=nil{
		log.Fatal(err)
	}

	for{
		reply ,err:=stream.Recv()
		if err!=nil{
			if err==io.EOF{
				break
			}
			log.Fatal(err)
		}
		fmt.Println(reply.GetValue())
	}
}
