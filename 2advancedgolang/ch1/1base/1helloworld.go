package main
import (
	"fmt"
	"log"
	"net/http"
"time"
)
func main(){
	fmt.Println("please visit http://localhost:12345")
	http.HandleFunc("/",func(w http.ResponseWriter,req *http.Request){
		s:= fmt.Sprintf("hello world! - - Time:%s",time.Now().String())
		fmt.Fprintf(w,"%v\n",s)
		log.Println("%v\n",s)
	})
	if err:=http.ListenAndServe(":12345",nil);err!=nil{
		log.Fatal("listenandserve: ",err)
	}
}