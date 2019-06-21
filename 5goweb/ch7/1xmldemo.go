package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
)

type RecurlyServers struct {
	XmlName     xml.Name `xml:"servers"`
	Version     string   `xml:"version,attr"`
	Svs         []server `xml:"server"`
	Description string   `xml:",innerxml"`
}

type server struct {
	XmlName    xml.Name `xml:"server"`
	ServerName string   `xml:"serverName"`
	ServerIP   string   `xml:"serverIP"`
}

func main() {
	file, err := os.Open("servers.xml")
	if err != nil {
		fmt.Printf("error: %v", err)
		return
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Printf("error %v", err)
		return
	}
	v := RecurlyServers{}
	err = xml.Unmarshal(data, &v)
	if err != nil {
		fmt.Printf("error %v", err)
		return
	}
	fmt.Println(v)
}
