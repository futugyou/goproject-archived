package main

import (
	context "context"
	"log"
	"net"
	"strings"
	"time"

	pub "work/golang-test/advancedgolang/ch4/4-4/pubsubservice"

	"github.com/docker/docker/pkg/pubsub"
	grpc "google.golang.org/grpc"
)

type PubsubService struct {
	pub *pubsub.Publisher
}

func NewPubsubService() *PubsubService {
	return &PubsubService{pub: pubsub.NewPublisher(100*time.Millisecond, 10)}
}

func (p *PubsubService) Publish(ctx context.Context, arg *pub.String) (*pub.String, error) {
	p.pub.Publish(arg.GetValue())
	return &pub.String{},nil
}

func (p *PubsubService) Subscribe(arg *pub.String, stream pub.PubsubService_SubscribeServer) error {
	ch := p.pub.SubscribeTopic(func(v interface{}) bool {
		if key, ok := v.(string); ok {
			if strings.HasPrefix(key, arg.GetValue()) {
				return true
			}
		}
		return false
	})
	for v := range ch {
		if err := stream.Send(&pub.String{Value: v.(string)}); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	grpcServer := grpc.NewServer()
	pub.RegisterPubsubServiceServer(grpcServer, NewPubsubService())

	lis, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal(err)
	}
	grpcServer.Serve(lis)
}
