package main

import (
	context "context" 
	"net"
	gw "work/golang-test/advancedgolang/ch4/4-6/proto" 
	grpc "google.golang.org/grpc"
)

type RestServiceImpl struct {
}

func (p *RestServiceImpl) Get(ctx context.Context, message *gw.StringMessage) (*gw.StringMessage, error) {
	return &gw.StringMessage{Value: "get hi:" + message.Value + "#"}, nil
}
func (p *RestServiceImpl) Post(ctx context.Context, message *gw.StringMessage) (*gw.StringMessage, error) {
	return &gw.StringMessage{Value: "post hi:" + message.Value + "@"}, nil
}

func main() {
	grpcServer:=grpc.NewServer()

	gw.RegisterRestServiceServer(grpcServer,new(RestServiceImpl))
	lis,_:=net.Listen("tcp",":5000")
	grpcServer.Serve(lis)
}
