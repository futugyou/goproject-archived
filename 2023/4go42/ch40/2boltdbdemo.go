package main

import (
	"bytes"
	"fmt"
	"log"
	"time"

	"github.com/boltdb/bolt"
)

func main() {
	Boltdb()
}
func Boltdb() error {
	db, err := bolt.Open("my.db", 0600, &bolt.Options{Timeout: 10 * time.Second})
	defer db.Close()
	if err != nil {
		log.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte("mybucket"))
		err = b.Put([]byte("answer"), []byte("42"))
		err = b.Put([]byte("why"), []byte("101010"))
		return err
	})

	err = db.Batch(func(tx *bolt.Tx) error {
		return nil
	})

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("mybucket"))
		v := b.Get([]byte("answer"))
		id, _ := b.NextSequence()
		fmt.Printf("the answer is : %s %d \n", v, id)

		c := b.Cursor()
		fmt.Println("\n cursor key ")
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("key=%s,value=%s\n", k, v)
		}

		fmt.Println("\nprefix ")
		prefix := []byte("a")
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			fmt.Printf("key=%s,value=%s\n", k, v)
		}
		return nil
	})

	db.View(func(tx *bolt.Tx) error {
		fmt.Println("\nForeach() ")
		b := tx.Bucket([]byte("mybucket"))
		b.ForEach(func(k, v []byte) error {
			fmt.Printf("key=%s,value=%s\n", k, v)
			return nil
		})
		return nil
	})

	tx, err := db.Begin(true)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.CreateBucket([]byte("mybucket"))
	if err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	return err
}
