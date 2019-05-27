package main

import (
	"log"

	"github.com/zieckey/etcdsync"
)

func main() {
	m, err := etcdsync.New("/lock", 10, []string{"http://127.0.0.1:2379"})
	if m == null || err != nil {
		log.Printf("etcdsync.New failed")
		return
	}
	err = m.Lock()
	if err != nil {
		log.Printf("etcdsync.Lock failed")
		return
	}
	log.Printf("etcdsync.Lock ok")
	/////

	err = m.Unlock()
	if err != nil {
		log.Printf("etcdsync.Unlock failed")
		return
	}
	log.Printf("etcdsync.Unlock ok")
}
