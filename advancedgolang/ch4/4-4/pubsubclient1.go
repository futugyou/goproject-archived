package main

import (
	context "context"
	"log"

	sub "work/golang-test/advancedgolang/ch4/4-4/pubsubservice"

	grpc "google.golang.org/grpc"
)

func main() {
	conn, err := grpc.Dial("localhost:1234", grpc.WithInsecure())
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := sub.NewPubsubServiceClient(conn)

	_, err = client.Publish(context.Background(), &sub.String{Value: "golang: hello go"})
	if err != nil {
		log.Fatal(err)
	}
	_, err = client.Publish(context.Background(), &sub.String{Value: "docker: hello docker"})
	if err != nil {
		log.Fatal(err)
	}
}
