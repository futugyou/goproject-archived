package main

import (
	context "context"
	fmt "fmt"
	"io"
	"log"
	"time"
	hello "work/golang-test/advancedgolang/ch4/4-4/HelloService"

	grpc "google.golang.org/grpc"
)

type Authentication struct {
	Key   string
	Value string
}

func (a *Authentication) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"key": a.Key, "Value": a.Value}, nil
}

func (a *Authentication) RequireTransportSecurity() bool {
	return false
}
func main() {
	auth := Authentication{Key: "aaaaa", Value: "bbbbb"}
	conn, err := grpc.Dial("localhost:1234", grpc.WithInsecure(), grpc.WithPerRPCCredentials(&auth))
	if err != nil {
		log.Fatal(err)
	}

	defer conn.Close()

	client := hello.NewHelloServiceClient(conn)
	reply, err := client.Hello(context.Background(), &hello.String{Value: "Hello"})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(reply.GetValue())

	stream, err := client.Channel(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	go func() {
		for {
			if err := stream.Send(&hello.String{Value: "hi"}); err != nil {
				log.Fatal(err)
			}
			time.Sleep(time.Second)
		}
	}()
	for {
		reply, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			}
			log.Fatal(err)
		}
		fmt.Println(reply.GetValue())
	}

}
