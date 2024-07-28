package main

import (
	"fmt"
)

type Book struct {
	pages int
}

type Books []Book

func (books *Books) Modify() {
	(*books)[0].pages = 999
	*books = append(*books, Book{178})
}
func (books Books) Modify2() {
	(books)[0].pages = 999
	books = append(books, Book{178})
}
func main() {
	var books = Books{{123}, {234}}
	books.Modify2()
	fmt.Println(books)
	books.Modify()
	fmt.Println(books)
}
