package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	_ "github.com/denisenkom/go-mssqldb"
)

var db *sql.DB

var server = "thro.database.chinacloudapi.cn"
var port = 1433
var user = "btcapi"
var password = "!Qaz2wsx"
var database = "dealer_ordercenterdb"

func main() {
	connString := fmt.Sprintf("server=%s;user id=%s;password=%s;port=%d;database=%s;",
		server, user, password, port, database)

	var err error
	db, err = sql.Open("sqlserver", connString)
	if err != nil {
		log.Fatal("error creating connection pool: ", err.Error())
	}
	
	ctx := context.Background()
	err = db.PingContext(ctx)
	if err != nil {
		log.Fatal(err.Error())
	}
	fmt.Println("connected!") 
	createID, err := CreateEmployee("jake", "united states")
	if err != nil {
		log.Fatal("error creating employee: ", err.Error())
	}
	fmt.Printf("inserted id: %d successfully.\n", createID)

	count, err := ReadEmployees()
	if err != nil {
		log.Fatal("error reading employees: ", err.Error())
	}
	fmt.Printf("read %d row successfully.\n", count)

	updateRows, err := UpdateEmployee("jake", "poland")
	if err != nil {
		log.Fatal("error updating employees: ", err.Error())
	}
	fmt.Printf("updated %d row successfully.\n", updateRows)

	deletedRows, err := DeleteEmployee("jake")
	if err != nil {
		log.Fatal("error deleting employees: ", err.Error())
	}
	fmt.Printf("deleted %d row successfully.\n", deletedRows)
}

func CreateEmployee(name string, loc string) (int64, error) { 
	var err error
	if db == nil {
		err = errors.New("createemployee:db is null")
		return -1, err
	}
	ctx := context.Background()
	err = db.PingContext(ctx)
	if err != nil {
		return -1, err
	}

	tsql := "insert into DealerSchema.Employees(name,location) values(@name,@location); select convert(bigint,SCOPE_IDENTITY());"
	stmt, err := db.Prepare(tsql)
	if err != nil {
		return -1, err
	}
	defer stmt.Close()

	row := stmt.QueryRowContext(
		ctx,
		sql.Named("name", name),
		sql.Named("location", loc),
	)
	var newID int64
	err = row.Scan(&newID)
	if err != nil {
		return -1, err
	}
	return newID, nil
}

func ReadEmployees() (int, error) {
	ctx := context.Background()
	err := db.PingContext(ctx)
	if err != nil {
		return -1, err
	}
	tsql := fmt.Sprintf("select id ,name ,location from DealerSchema.Employees;")

	rows, err := db.QueryContext(ctx, tsql)
	if err != nil {
		return -1, err
	}
	defer rows.Close()

	var count int

	for rows.Next() {
		var name, location string
		var id int

		err = rows.Scan(&id, &name, &location)
		if err != nil {
			return -1, err
		}
		fmt.Printf("id: %d, name: %s, location: %s\n", id, name, location)
		count++
	}
	return count, nil
}

func UpdateEmployee(name string, loc string) (int64, error) {
	ctx := context.Background()
	err := db.PingContext(ctx)
	if err != nil {
		return -1, err
	}
	tsql := fmt.Sprintf("update DealerSchema.Employees set location = @location where name=@name")
	result, err := db.ExecContext(
		ctx,
		tsql,
		sql.Named("location", loc),
		sql.Named("name", name),
	)
	if err != nil {
		return -1, err
	}
	return result.RowsAffected()
}

func DeleteEmployee(name string) (int64, error) {
	ctx := context.Background()
	err := db.PingContext(ctx)
	if err != nil {
		return -1, err
	}
	tsql := fmt.Sprintf("delete from DealerSchema.Employees where name=@name")

	result, err := db.ExecContext(ctx, tsql, sql.Named("name", name))
	if err != nil {
		return -1, err
	}
	return result.RowsAffected()
}
