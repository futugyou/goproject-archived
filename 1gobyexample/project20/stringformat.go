package main
import(
	"fmt"
	"os"
)
var f= fmt.Printf
type point struct{
	x,y int
}
func main(){
p:=point{1,4}
f("%v\n",p)
f("%+v\n",p)
f("%#v\n",p)
f("%T\n",p)

f("%t\n",true)

f("%d\n",123)
f("%b\n",123)

f("%c\n",123)

f("%x\n",123)

f("%f\n",123.23)

f("%e\n",123400000.0)
f("%Ee\n",123400000.0)
fmt.Printf("%s\n", "\"string\"")
fmt.Printf("%q\n", "\"string\"")
fmt.Printf("%x\n", "hex this")
fmt.Printf("%p\n", &p)
fmt.Printf("|%6d|%6d|\n", 12, 345)
fmt.Printf("|%6.2f|%6.2f|\n", 1.2, 3.45)
fmt.Printf("|%-6.2f|%-6.2f|\n", 1.2, 3.45)
fmt.Printf("|%6s|%6s|\n", "foo", "b")
fmt.Printf("|%-6s|%-6s|\n", "foo", "b")
s := fmt.Sprintf("a %s", "string")
fmt.Println(s)
fmt.Fprintf(os.Stderr, "an %s\n", "error")
}