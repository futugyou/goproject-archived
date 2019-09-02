package main

import (
	"fmt"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

type Post struct {
	Id         int    `db:"id" json:"id"`
	Content    string `db:"content" json:"content"`
	AuthorName string `db:"author" json:"author"`
}

var db *sqlx.DB

func init() {
	var err error
	db, err = sqlx.Open("postgres", "user=postgres dbname=postgres password=1qaz2wsx sslmode=disable")
	if err != nil {
		panic(err)
	}
}

func GetPost(id int) (post []Post, err error) {
	post = []Post{}
	err = db.Select(&post, "select id ,content,author from posts ")
	//err = db.QueryRowx("select id ,content,author from posts where id =$1", id).StructScan(&post)
	if err != nil {
		panic(err)
	}
	return
}

func (post *Post) Create() (err error) {
	err = db.QueryRow("insert into posts (content,author) values ($1,$2) returning id", post.Content, post.AuthorName).Scan(&post.Id)
	return
}

func main() {
	post := Post{Content: "hello sqlx", AuthorName: "sqlx"}
	post.Create()
	fmt.Println(post)
	posts, _ := GetPost(post.Id)
	fmt.Println(posts)
}
