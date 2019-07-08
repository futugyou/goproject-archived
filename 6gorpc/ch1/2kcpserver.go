package main

import (
	"crypto/sha1"
	"net"
	arg "work/golang-test/6gorpc/ch1/service"

	"github.com/smallnest/rpcx/server"
	kcp "github.com/xtaci/kcp-go"
	"golang.org/x/crypto/pbkdf2"
)

const cryptKey = "rpcx-key"
const cryptSalt = "rpcx-salt"


//go run -tags kcp C:\Code\Golang\src\work\golang-test\6gorpc\ch1\2kcpserver.go
func main() {
	pass := pbkdf2.Key([]byte(cryptKey), []byte(cryptSalt), 4096, 32, sha1.New)
	bc, err := kcp.NewAESBlockCrypt(pass)
	if err != nil {
		panic(err)
	}
	s := server.NewServer(server.WithBlockCrypt(bc))
	s.RegisterName("Arith", new(arg.Arith), "")

	cs := &ConfigUDPSession{}
	s.Plugins.Add(cs)

	err = s.Serve("kcp", ":7890")
	if err != nil {
		panic(err)
	}
}

type ConfigUDPSession struct{}

func (p *ConfigUDPSession) HandleConnAccept(conn net.Conn) (net.Conn, bool) {
	session, ok := conn.(*kcp.UDPSession)
	if !ok {
		return conn, true
	}

	session.SetACKNoDelay(true)
	session.SetStreamMode(true)
	return conn, true
}
