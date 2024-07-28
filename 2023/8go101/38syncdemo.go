package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {

	mutex2 := sync.Mutex{}
	//使用锁创建一个条件等待
	cond2 := sync.NewCond(&mutex2)

	for i := 0; i < 10; i++ {
		go printNum(i, cond2)
	}
	time.Sleep(time.Second * 4)
	fmt.Println("-------------------")
	//等待一秒后，我们先唤醒一个等待，输出一个数字
	cond2.L.Lock()
	cond2.Signal()
	cond2.L.Unlock()

	time.Sleep(time.Second * 4)
	fmt.Println("-------------------")
	//再次待待一秒后，唤醒所有，输出余下四个数字
	cond2.L.Lock()
	cond2.Broadcast()
	cond2.L.Unlock()
}

func printNum(num int, cond *sync.Cond) {
	cond.L.Lock()
	if num < 5 {
		//num小于5时，进入等待状态
		cond.Wait()
	}
	//大于5的正常输出
	fmt.Println(num)
	cond.L.Unlock()
}
