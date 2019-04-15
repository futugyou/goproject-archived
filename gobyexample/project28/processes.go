package main

import (
	"fmt"
	"os"
	"syscall"

	//"fmt"
	//"io/ioutil"
	"os/exec"
	"os/signal"
)

func main() {
	// datecmd := exec.Command("date")

	// dateout, err := datecmd.Output()
	// if err != nil {
	// 	panic(err)
	// }

	//fmt.Print("> date")
	//fmt.Println(string(dateout))

	// grepcmd := exec.Command("grep", "hello")

	// grepin, _ := grepcmd.StdinPipe()
	// grepout, _ := grepcmd.StdoutPipe()
	// grepcmd.Start()
	// grepin.Write([]byte("hello grep\ngoodbye grep"))
	// grepin.Close()
	// grepbytes, _ := ioutil.ReadAll(grepout)
	// grepcmd.Wait()
	// fmt.Println("> grep hello")
	// fmt.Println(string(grepbytes))

	// lsCmd := exec.Command("bash", "-c", "ls -a -l -h")
	// lsOut, err := lsCmd.Output()
	// if err != nil {
	//     panic(err)
	// }
	// fmt.Println("> ls -a -l -h")
	// fmt.Println(string(lsOut))

	b, _ := exec.LookPath("ls")
	args := []string{"ls", "-a", "-l", "-h"}
	env := os.Environ()
	execerr := syscall.Exec(b, args, env)
	if execerr != nil {
		//panic(execerr)
	}

	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println()
		fmt.Println(sig)
		done <- true
	}()

	fmt.Println("awaiting sigal")
	<-done
	fmt.Println("exiting")

	//nerver run this code 
	defer fmt.Println("!")
	os.Exit(3)
}
