package main

import (
	"crypto/md5"
	"crypto/sha1"
	b64 "encoding/base64"
	"fmt"
	"net/url"
	"strings"
)

func main() {
	p := fmt.Println
	s := "postgres://user:pass@host.com:5432/path?k=v#f"
	u, err := url.Parse(s)
	if err != nil {
		p(err)
	}
	p(u.Scheme)
	p(u.User)
	p(u.User.Username())
	pa, _ := u.User.Password()
	p(pa)
	p(u.Host)
	h := strings.Split(u.Host, ":")
	p(h)
	p(u.Path)
	p(u.Fragment)
	p(u.RawQuery)
	m, _ := url.ParseQuery(u.RawQuery)
	p(m)
	p(m["k"][0])

	sha := sha1.New()
	sha.Write([]byte(s))
	bs := sha.Sum(nil)
	p(s)
	fmt.Printf("%x\n", bs)

	md := md5.New()
	md.Write([]byte(s))
	mds := md.Sum(nil)
	fmt.Printf("%x\n", mds)

	date := "abc123!?$*&()'-=@~"
	senc := b64.StdEncoding.EncodeToString([]byte(date))
	p(senc)

	sdnc, _ := b64.StdEncoding.DecodeString(senc)
	p(string(sdnc))

	uenc := b64.URLEncoding.EncodeToString([]byte(date))
	p(uenc)
	udnc, _ := b64.URLEncoding.DecodeString(uenc)
	p(string(udnc))

}
