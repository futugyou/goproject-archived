package main

import (
	"fmt"
	"os"

	"github.com/futugyou/openclaw/core"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	// test migrate
	dsn := os.Getenv("PostresDB_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	db.AutoMigrate(&core.AutomationDefinition{})
}
