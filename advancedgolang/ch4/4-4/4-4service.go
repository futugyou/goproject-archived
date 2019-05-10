package main

import (
	"context"
)

type HelloServiceImpl struct{}

func (p *HelloServiceImpl) Hello(ctx context.Context, args *String) (*String, error) {
	reply := &String{value: "hello:" + args.GetValue()}
	return reply, nil
}
