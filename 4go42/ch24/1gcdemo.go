package main

import (
	"log"
	"runtime"
	"time"
)

type Person struct {
	Name string
	Age  int
}

func (p *Person) Close() {
	p.Name = "NewName"
	log.Println(p)
	log.Println("Close")
}
func (p *Person) NewOpen() {
	log.Println("init")
	runtime.SetFinalizer(p, (*Person).Close)
}

func Tt(p *Person) {
	p.Name = "NewName"
	log.Println(p)
	log.Println("Tt")
}

func Mem(m *runtime.MemStats) {
	runtime.ReadMemStats(m)
	log.Printf("%d kb\n", m.Alloc/1024)
}

func main() {
	var m runtime.MemStats
	Mem(&m)

	var p *Person = &Person{Name: "lee", Age: 44}
	p.NewOpen()
	log.Println("GC complete first time")
	log.Println("p:", p)
	runtime.GC()
	time.Sleep(time.Second * 5)
	Mem(&m)

	var p1 *Person = &Person{Name: "tom", Age: 22}
	runtime.SetFinalizer(p1, Tt)
	log.Println("GC complete second time")
	time.Sleep(time.Second * 2)
	runtime.GC()
	time.Sleep(time.Second * 2)
	Mem(&m)
}
