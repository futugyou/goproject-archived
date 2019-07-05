package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

func main() {
	for skip := 0; ; skip++ {
		pc, file, line, ok := runtime.Caller(skip)
		if !ok {
			break
		}

		p := runtime.FuncForPC(pc)
		fnfile, fnline := p.FileLine(0)
		fmt.Printf("skip = %d, pc = 0x%08X\n", skip, pc)
		fmt.Printf("  func: file = %s, line = L%03d, name = %s, entry = 0x%08X\n", fnfile, fnline, p.Name(), p.Entry())
		fmt.Printf("  call: file = %s, line = L%03d\n\n", file, line)
		var buf = make([]byte, 64)
		var stk = buf[:runtime.Stack(buf, true)]
		print(string(stk))
		fmt.Println()
		var goid =GetGoid()
		fmt.Println(goid)
	}
}
func GetGoid() int64 {
	var (
		buf [64]byte
		n   = runtime.Stack(buf[:], false)
		stk = strings.TrimPrefix(string(buf[:n]), "goroutine ")
	)
	idField := strings.Fields(stk)[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Errorf("can not get goroutine id: %v", err))
	}

	return int64(id)
}
