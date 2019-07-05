package main

import (
	context "context"
	"fmt"
	"io"
	"log"
	"net"
	hello "work/golang-test/advancedgolang/ch4/4-4/HelloService"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
)

type HelloServiceImpl struct{ auth *Authentication }

func (p *HelloServiceImpl) Hello(ctx context.Context, args *hello.String) (*hello.String, error) {
	p.auth = &Authentication{Key: "aaaaa", Value: "bbbbb"}
	if err := p.auth.Auth(ctx); err != nil {
		return nil, err
	}
	reply := &hello.String{Value: "hello:" + args.GetValue()}
	return reply, nil
}

func (p *HelloServiceImpl) Channel(stream hello.HelloService_ChannelServer) error {

	for {
		args, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		reply := &hello.String{Value: "hello : " + args.GetValue()}

		err = stream.Send(reply)
		if err != nil {
			return err
		}
	}
}

type Authentication struct {
	Key   string
	Value string
}

func (a *Authentication) Auth(ctx context.Context) error {
	md, ok := metadata.FromIncomingContext(ctx)

	if !ok {
		return fmt.Errorf("missing credentials")
	}

	var appid string
	var appkey string

	if val, ok := md["key"]; ok {
		appid = val[0]
	}
	if val, ok := md["value"]; ok {
		appkey = val[0]
	}

	if appid != a.Key || appkey != a.Value {
		return grpc.Errorf(codes.Unauthenticated, "invalid token")
	}
	return nil
}

func filter(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
	log.Println("fileter:", info)
	return handler(ctx, req)
}
func channelFilter(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error{
	log.Printf("before handling. Info: %+v", info)
	err := handler(srv, ss)
	log.Printf("after handling. err: %v", err)
	return err
}
func main() {
	grpcServer := grpc.NewServer(
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			filter, 
		)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			channelFilter, 
		)),
		)
	hello.RegisterHelloServiceServer(grpcServer, new(HelloServiceImpl))

	lis, err := net.Listen("tcp", ":1234")
	if err != nil {
		log.Fatal(err)
	}
	grpcServer.Serve(lis)
}
