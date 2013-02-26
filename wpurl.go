package main

import (
	"fmt"
	"labix.org/v2/mgo"
	"net/http"
	"time"
)

type UrlInfo struct {
	Mtime time.Time
}

func main() {
	session, err := mgo.Dial("localhost")
	if err != nil {
		panic(err)
	}
	c := session.DB("wp").C("urls")
	resp, err := http.Head("http://www.nasa.gov")
	defer resp.Body.Close()
	if err != nil {
		panic(err)
	}
	t, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
	if err != nil {
		panic(err)
	}
	ui := UrlInfo{
		Mtime: t}
	err = c.Insert(&ui)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%v\n", t)
}
