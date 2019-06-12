// 数据库初始化，建立连接
package model

import (
	"database/sql"

	"github.com/go-redis/redis"

	_ "github.com/denisenkom/go-mssqldb"
)

type DbWorker struct {
	dsn string
	Db  *sql.DB
}

func NewDb() DbWorker {
	dbw := DbWorker{dsn: "Server=turbo.database.chinacloudapi.cn; database=dealerlab;User ID=btcapi;Password=!Qaz2wsx;"}
	dbtemp, _ := sql.Open("mssql", dbw.dsn)
	dbw.Db = dbtemp
	return dbw
}

func NewRedis() *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	return client
}
