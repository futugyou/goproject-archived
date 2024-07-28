package service

import "context"

type Args struct {
	A int
	B int
}

type Reply struct {
	C int
}

type Arith int

func (a *Arith) Mul(ctx context.Context, args *Args, r *Reply) error {
	r.C = args.A * args.B
	return nil
}
