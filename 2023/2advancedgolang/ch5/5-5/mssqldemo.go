package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/denisenkom/go-mssqldb"
)

func main() {
	db, err := sql.Open("mssql", "Server=***; database=testdb;User ID=sa;Password=!Qaz2wsx;")
	if err != nil {
		fmt.Println(" Error open db:", err.Error())
	}

	var (
		sqlversion string
	)

	rows, err := db.Query("select @@version")
	if err != nil {
		log.Fatal(err)
	}
	for rows.Next() {
		err := rows.Scan(&sqlversion)
		if err != nil {
			log.Fatal(err)
		}
		log.Println(sqlversion)
	}

	// 执行SQL语句
	rows, err = db.Query("select name from sys.tables ")

	if err != nil {
		fmt.Println("query: ", err)
		return
	}
	for rows.Next() {
		var name string
		rows.Scan(&name)
		fmt.Printf("tablename: %s \n", name)
	}
}
