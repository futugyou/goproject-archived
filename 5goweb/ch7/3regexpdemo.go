package main

import (
	"fmt"
	"regexp"
)

func main() {
	a := "I am learning Go language"
	re, _ := regexp.Compile("[a-z]{2,4}")
	one := re.Find([]byte(a))
	fmt.Println("find:", string(one))

	all := re.FindAll([]byte(a), -1)
	fmt.Println("find all:", all)

	index := re.FindIndex([]byte(a))
	fmt.Println("FindIndex:", index)

	indexAll := re.FindAllIndex([]byte(a), -1)
	fmt.Println("findallindex:", indexAll)

	re2, _ := regexp.Compile("am(.*)lang(.*)")
	submatch := re2.FindSubmatch([]byte(a))
	for _, v := range submatch {
		fmt.Println(string(v))
	}

	submatchindex := re2.FindSubmatchIndex([]byte(a))
	fmt.Println("FindSubmatchIndex:", submatchindex)

	submatchall := re2.FindAllSubmatch([]byte(a), -1)
	fmt.Println("FindAllSubmatch:", submatchall)

	submatchallindex := re2.FindAllSubmatchIndex([]byte(a), -1)
	fmt.Println("FindAllSubmatchIndex:", submatchallindex)

	src := []byte(`
	call hello alice
	hello bob
	call hello eve	
	`)

	pat := regexp.MustCompile(`(?m)(call)\s+(?P<cmd>\w+)\s+(?P<arg>.+)\s*$`)
	res := []byte{}
	for _, s := range pat.FindAllSubmatchIndex(src, -1) {
		res = pat.Expand(res, []byte("$cmd('$arg')\n"), src, s)
	}
	fmt.Println(string(res))
}
