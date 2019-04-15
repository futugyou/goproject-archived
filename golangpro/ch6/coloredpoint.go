package main

import (
	"fmt"
	"time"
	"bytes"
)

type Rocket struct { /* ... */
}

func (r *Rocket) Launch() { /* ... */ }
func main() {
	r := new(Rocket)
	time.AfterFunc(10*time.Second, r.Launch)

	var x IntSet
	x.Add(1)
	x.Add(144)
	x.Add(9)
	fmt.Println(x.String())
}

type IntSet struct {
	words []uint64
}

func (s *IntSet) Has(x int) bool {
	word, bit := x/64, uint(x%64)
	fmt.Printf("word :%d , bit :%d , 1<<bit :%d\n", word, bit,1<<bit)
	return word < len(s.words) && s.words[word]&(1<<bit) != 0
}

func (s *IntSet) Add(x int) {
	word, bit := x/64, uint(x%64)
	fmt.Printf("word :%d , bit :%d , 1<<bit :%d\n", word, bit,1<<bit)
	for word >= len(s.words) {
		s.words = append(s.words, 0)
	}
	s.words[word] |= 1 << bit
}

func (s *IntSet) UnionWith(t *IntSet) {
	for i, tword := range t.words {
		if i < len(s.words) {
			s.words[i] |= tword
		} else {
			s.words = append(s.words, tword)
		}
	}
}

func (s *IntSet) String() string {
	var buf bytes.Buffer
	buf.WriteByte('{')
	for i,word := range s.words {
		if word==0{
			continue
		}
		for j:=0;j<64;j++{
			if word&(1<<uint(j))!=0{
				if buf.Len()>len("{"){
					buf.WriteByte(' ')
				}
				fmt.Fprintf(&buf,"%d",64*i+j)
			}
		}
	}
	buf.WriteByte('}')
	return buf.String()
}