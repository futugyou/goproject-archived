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
	c:=client.NewClient(client.DefaultOption)
	err:=c.Connect("tcp","127.0.0.1:8972")
	if err!=nil{
		panic(err)
	}

	defer c.Close()

	args:=&arg.Args{
		A:10,
		B:20,
	}

	reply:=&arg.Reply{}
	err=c.Call(context.Background(),"Arith","Mul",args,reply)
	if err!=nil{
		log.Fatalf("failed to call :%v",err)
	}

	log.Printf("%d * %d = %d",args.A,args.B,reply.C)

	ch:=make(chan *protocol.Message)
	c.RegisterServerMessageChan(ch)

	for meg:=range ch{
		fmt.Printf("receive msg from server :%s , time %s\n",meg.Payload,time.Now())
	}
}