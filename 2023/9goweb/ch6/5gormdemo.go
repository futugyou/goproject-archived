package main

import (
	"fmt"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
)

type Sender struct {
	Id       int
	Content  string
	Author   string    `gorm:"not null"`
	Comments []Comment `gorm:"FOREIGNKEY:SenderId;ASSOCIATION_FOREIGNKEY:Id"`
	CreateAt time.Time
}

type Comment struct {
	Id       int
	Content  string
	Author   string `gorm:"not null"`
	SenderId int    `gorm:"index"`
	CreateAt time.Time
}

var db *gorm.DB

func init() {
	var err error
	db, err = gorm.Open("postgres", "user=postgres dbname=postgres password=1qaz2wsx sslmode=disable")
	if err != nil {
		panic(err)
	}

	db.AutoMigrate(&Sender{}, &Comment{})
}

func main() {
	send := Sender{Content: "hello world", Author: "sender"}
	db.Create(&send)
	fmt.Println(send)

	comment := Comment{Content: "one!", Author: "a"}
	ass := db.Model(&send).Association("Comments") //.Append(comment)
	err := ass.Error
	if err != nil {
		panic(err)
	}
	ass.Append(comment)
	var readSend Sender
	db.Where("author = $1", "sender").First(&readSend)
	var comments []Comment
	db.Model(&readSend).Related(&comments)
	fmt.Println(comments)
}
