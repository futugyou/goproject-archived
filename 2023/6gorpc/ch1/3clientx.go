package main

import(
	"context"
	"fmt"
	"log"
	arg "work/golang-test/6gorpc/ch1/service"
	"github.com/smallnest/rpcx/client"
	"github.com/smallnest/rpcx/protocol"
	"time"
)

func main(){
	ch:=make(chan *protocol.Message)

	d:=client.NewPeer2PeerDiscovery("tcp@127.0.0.1:8972","")
	xclient:=client.NewBidirectionalXClient("Arith", client.Failtry, client.RandomSelect, d, client.DefaultOption, ch)
	defer xclient.Close()

	args:=&arg.Args{
		A:10,
		B:20,
	}

	reply:=&arg.Reply{}

	err:=xclient.Call(context.Background(),"Mul",args,reply)
	if err!=nil{
		log.Fatalf("failed to call :%v",err)
	}	
	log.Printf("%d * %d = %d",args.A,args.B,reply.C)

	
	for meg:=range ch{
		fmt.Printf("receive msg from server :%s , time %s\n",meg.Payload,time.Now())
	}
}