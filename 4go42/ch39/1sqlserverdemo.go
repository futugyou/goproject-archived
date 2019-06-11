package main

import (
	"database/sql"
	"fmt"
	_ "log"
	"strings"
	_ "strings"
	"time"

	_ "github.com/denisenkom/go-mssqldb"
)

type DbWorker struct {
	Dsn string
	Db  *sql.DB
}
type Cate struct {
	cid     int
	cname   string
	addtime time.Time
	scope   int
}

func main() {
	//死活连不上本地数据库
	//dbw := DbWorker{Dsn: `"Data Source=(LocalDb)\MSSQLLocalDB;Initial Catalog=CampaignDB;Persist Security Info=True;User ID=sa"`}
	dbw := DbWorker{Dsn: "Server=turbo.database.chinacloudapi.cn; database=dealerlab;User ID=btcapi;Password=!Qaz2wsx;"}
	dbtemp, err := sql.Open("mssql", dbw.Dsn)
	dbw.Db = dbtemp
	if err != nil {
		panic(err)
		return
	}
	defer dbw.Db.Close()

	//dbw.insertData()
	//dbw.deleteData()
	//dbw.editData()
	//dbw.queryData()
	dbw.transaction()
}

func (dbw *DbWorker) insertData() {
	stmt, _ := dbw.Db.Prepare(`insert into t_article_cate (cname, addtime,scope) values(?,?,?)`)
	defer stmt.Close()
	ret, err := stmt.Exec("text1", time.Now(), 10)
	if err != nil {
		fmt.Printf("insert data error :%v\n", err)
		return
	}

	if LastInsertId, err := ret.LastInsertId(); err == nil {
		fmt.Println("LastInsertId:", LastInsertId)
	}

	if RowsAffected, err := ret.RowsAffected(); nil == err {
		fmt.Println("RowsAffected:", RowsAffected)
	}
}

func (dbw *DbWorker) deleteData() {
	stmt, err := dbw.Db.Prepare(`delete from t_article_cate where cid=?`)
	defer stmt.Close()
	ret, err := stmt.Exec(1)
	if err != nil {
		fmt.Println("insert data error : %v\n", err)
	}
	if RowsAffected, err := ret.RowsAffected(); nil == err {
		fmt.Println("RowsAffected:", RowsAffected)
	}
}

func (dbw *DbWorker) editData() {
	stmt, err := dbw.Db.Prepare(`update t_article_cate set scope = ? where cid=?`)
	defer stmt.Close()
	ret, err := stmt.Exec(111, 12)
	if err != nil {
		fmt.Println("insert data error : %v\n", err)
	}
	if RowsAffected, err := ret.RowsAffected(); nil == err {
		fmt.Println("RowsAffected:", RowsAffected)
	}
}

func (dbw *DbWorker) queryData() {
	//rows,err:=db.Query(`select cid ,cname from t_article_cate where cid=?`,1)
	//err= db.QueryRow("select cname from t_article_cate where id=?",1).Scan(&name)
	stmt, _ := dbw.Db.Prepare(`SELECT cid, cname, addtime, scope From t_article_cate where status=?`)
	defer stmt.Close()

	rows, err := stmt.Query(0)
	defer rows.Close()
	if err != nil {
		fmt.Printf("insert data error: %v\n", err)
		return
	}

	columns, _ := rows.Columns()
	fmt.Println(columns)
	rowMaps := make([]map[string]string, 9)
	values := make([]sql.RawBytes, len(columns))
	scans := make([]interface{}, len(columns))
	for i := range values {
		scans[i] = &values[i]
	}
	i := 0
	for rows.Next() {
		err = rows.Scan(scans...)
		each := make(map[string]string, 4)
		for i, col := range values {
			each[columns[i]] = string(col)
		}
		rowMaps = append(rowMaps[:i], each)
		fmt.Println(each)
		i++
	}
	fmt.Println(rowMaps)
	for i, col := range rowMaps {
		fmt.Println(i, col)
	}

	err = rows.Err()
	if err != nil {
		fmt.Printf(err.Error())
	}
}

func (dbw *DbWorker) transaction() {
	tx, err := dbw.Db.Begin()
	if err != nil {
		fmt.Printf("transaction error : %v\n", err)
		return
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`insert into t_article_cate (cname, addtime,scope) values(?,?,?)`)
	if err != nil {
		fmt.Printf("insert error :%v\n", err)
		return
	}
	for i := 100; i < 140; i++ {
		cname := strings.Join([]string{"text-", string(i)}, "-")
		_, err = stmt.Exec(cname, time.Now(), i+10)
		if err != nil {
			fmt.Printf("insert data error : %v\n", err)
			return
		}
	}
	err = tx.Commit()
	if err != nil {
		fmt.Printf("commit data error : %v\n", err)
	}
	stmt.Close()
}
