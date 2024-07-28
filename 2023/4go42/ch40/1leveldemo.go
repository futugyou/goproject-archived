package main

import (
	"crypto/md5"
	"fmt"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

var md = md5.New()

func Read(db *leveldb.DB, num int) {
	var kStr string
	var haskKey string
	kStr = strconv.Itoa(num)
	md.Write([]byte(kStr))
	haskKey = fmt.Sprintf("%x", md.Sum(nil))
	md.Reset()

	db.Get([]byte(haskKey), nil)
}

func Write(db *leveldb.DB, num int) {
	var kStr string
	var haskKey string
	kStr = strconv.Itoa(num)
	md.Write([]byte(kStr))
	haskKey = fmt.Sprintf("%x", md.Sum(nil))
	md.Reset()

	db.Put([]byte(haskKey), []byte(kStr), nil)
}

func main() {
	db, _ := leveldb.OpenFile("levdb", nil)
	defer db.Close()

	_ = db.Put([]byte("key1"), []byte("1234556"), nil)
	_ = db.Put([]byte("key2"), []byte("qwertyu"), nil)
	_ = db.Put([]byte("key3"), []byte("asdfgh"), nil)
	_ = db.Put([]byte("key4"), []byte("zxcvbn"), nil)
	_ = db.Put([]byte("time"), []byte("0okm9ijn"), nil)

	data, _ := db.Get([]byte("key1"), nil)
	fmt.Println("key1=>", string(data))
	fmt.Println("1")
	_ = db.Delete([]byte("key"), nil)

	iter := db.NewIterator(nil, nil)
	for iter.Next() {
		key := iter.Key()
		value := iter.Value()
		fmt.Println(string(key), "=>", string(value))
	}
	iter.Release()
	iter.Error()
	fmt.Println("2")

	iter = db.NewIterator(nil, nil)
	//Seek: get key >="key4"
	for ok := iter.Seek([]byte("key4")); ok; ok = iter.Next() {
		fmt.Println(string(iter.Key()), "=>", string(iter.Value()))
	}
	iter.Release()
	fmt.Println("3")

	iter = db.NewIterator(&util.Range{Start: []byte("key"), Limit: []byte("t")}, nil)
	for iter.Next() {
		fmt.Println(string(iter.Key()), "=>", string(iter.Value()))
	}
	iter.Release()
	fmt.Println("4")

	iter = db.NewIterator(util.BytesPrefix([]byte("tim")), nil)
	for iter.Next() {
		fmt.Println(string(iter.Key()), "=>", string(iter.Value()))
	}
	iter.Release()
	_ = iter.Error()
	fmt.Println("5")

	batch := new(leveldb.Batch)
	var kStr string
	var batchkey string
	for i := 0; i < 10; i++ {
		kStr = strconv.Itoa(i)
		md.Write([]byte(kStr))
		batchkey = fmt.Sprintf("%x", md.Sum(nil))
		batch.Put([]byte(batchkey), []byte(kStr))
	}
	md.Reset()
	batch.Delete([]byte("lazy"))
	_ = db.Write(batch, nil)
}
