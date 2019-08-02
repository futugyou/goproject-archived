package main

import (
	"fmt"
	"reflect"
)

func main() {
	c := make(chan int, 1)
	vc := reflect.ValueOf(c)
	success := vc.TrySend(reflect.ValueOf(123))
	fmt.Println(success, vc.Len(), vc.Cap())

	vSend, vZero := reflect.ValueOf(789), reflect.Value{}
	branches := []reflect.SelectCase{
		{Dir: reflect.SelectDefault, Chan: vZero, Send: vZero},
		{Dir: reflect.SelectRecv, Chan: vc, Send: vZero},
		{Dir: reflect.SelectSend, Chan: vc, Send: vSend},
	}

	selIndex, vRecv, closed := reflect.Select(branches)
	vc.TrySend(reflect.ValueOf(333))
	fmt.Println(selIndex, vRecv.Int(), closed)
	vc.Close()
	selIndex, vRecv, closed = reflect.Select(branches[:2])
	fmt.Println(selIndex, vRecv.Int(), closed)

}
