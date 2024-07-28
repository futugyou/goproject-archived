package main

import (
	"fmt"
)

type Book struct {
	pages int
}

func (b Book) Pages() int { //Book.Pages(Book)
	return b.pages
}
func (b *Book) SetPages(pages int) { //(*Book).SetPages(*Book, int)
	b.pages = pages
}

func main() {
	var ss = make(map[string]struct{})
	_ = ss["key"] //no panic
	var book Book
	(*Book).SetPages(&book, 90)
	fmt.Println(Book.Pages(book))
}
