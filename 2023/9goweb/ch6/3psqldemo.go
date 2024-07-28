package main

import (
	"database/sql"
	"fmt"

	_ "github.com/lib/pq"
)

type Post struct {
	Id      int
	Content string
	Author  string
}

var db *sql.DB

func init() {
	var err error
	db, err = sql.Open("postgres", "user=postgres dbname=postgres password=1qaz2wsx sslmode=disable")
	if err != nil {
		panic(err)
	}
}

func Posts(limit int) (posts []Post, err error) {
	rows, err := db.Query("select id,content,author from posts limit $1", limit)
	if err != nil {
		return
	}
	for rows.Next() {
		post := Post{}
		err = rows.Scan(&post.Id, &post.Content, &post.Author)
		if err != nil {
			return
		}
		posts = append(posts, post)
	}
	rows.Close()
	return
}

func GetPost(id int) (post Post, err error) {
	post = Post{}
	err = db.QueryRow("select id ,content,author from posts where id =$1", id).Scan(&post.Id, &post.Content, &post.Author)
	return
}

func (post *Post) Create() (err error) {
	statement := "insert into posts (content,author) values ($1,$2) returning id"
	stmt, err := db.Prepare(statement)
	if err != nil {
		return
	}
	defer stmt.Close()
	err = stmt.QueryRow(post.Content, post.Author).Scan(&post.Id)
	return
}

func (post *Post) Update() (err error) {
	_, err = db.Exec("update posts set content = $2 ,author = $3 where id = $1", post.Id, post.Content, post.Author)
	return
}

func (post *Post) Delete() (err error) {
	_, err = db.Exec("delete from posts where id = $1", post.Id)
	return
}

func main() {
	post := Post{Content: "hello 1", Author: "bb"}
	fmt.Println(post.Create())
	fmt.Println(post)
	readPost, _ := GetPost(post.Id)
	readPost.Content = "like"
	readPost.Author = "user"
	readPost.Update()

	posts, _ := Posts(5)
	fmt.Println(posts)

	readPost.Delete()
}
