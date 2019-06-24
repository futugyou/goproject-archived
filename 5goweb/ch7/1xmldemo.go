package main

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
)

type RecurlyServers struct {
	XMLName     xml.Name `xml:"servers"`
	Version     string   `xml:"version,attr"`
	Svs         []server `xml:"server"`
	Description string   `xml:",innerxml"`
}

type server struct {
	XMLName    xml.Name `xml:"server"`
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

	fmt.Println("----------------------------")

	d := &RecurlyServers{Version: "1.0"}
	d.Svs = append(v.Svs, server{ServerName:"shanghai_vpn", ServerIP:"127.0.0.1"})
	d.Svs = append(v.Svs, server{ServerName:"beijing_vpn",ServerIP: "127.0.0.2"})
	output, err := xml.MarshalIndent(d, " ", " ")
	if err != nil {
		fmt.Printf("error: %v\n", err)
		return
	}
	os.Stdout.Write([]byte(xml.Header))
	os.Stdout.Write(output)
}
