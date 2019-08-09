package main
imports(
	"fmt"
	"time"
)

func longRunningWrong(message<-chan string)  {
	for{
		select {
		case <time.After(time.Minute):
			return 
		case msg:=<-message:
			fmt.Println(msg)
		}
	}
}

func longRunningRight(message<-chan string)  {
	timer:=time.NewTicker(time.Minute)
	defer timer.Stop()

	for{
		select {
		case <-timer.C:
			return 
		case msg:=<-message:
			fmt.Println(msg)
			if !timer.Stop() {
				<-timer.C
			}
			
		}
		timer.Reset(time.Minute)
	}
}