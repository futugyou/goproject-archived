package main
import "fmt"
func main()  {
	var a[5] int
	fmt.Println(a)
	fmt.Println(len(a))
	a[4]=100
	fmt.Println(a)
	fmt.Println(a[4])

	var twoD[2][3]int
	for i := 0; i < 2; i++ {
		for  j := 0; j < 3; j++  {
			twoD[i][j]=i+j
		}	
	}
	fmt.Println(twoD)
}