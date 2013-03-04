package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"
	"wp/db"
)

var lastModified time.Time
var contentLength int64

type UrlInfo struct {
	Content_Length int64
	Content_Type   string
	Etag           string
	Location       string
	Mtime          time.Time
	Path           string
	Status         string
	Status_Code    int
}

func main() {
	target := "http://www.nasa.gov/topics/nasalife/features/worldbook.html"
	// target = "http://www.nasa.gov/worldbook/index.html"
	urlinfo, err := url.Parse(target)
	if err != nil {
		panic(err)
	}
	/// get the collection connection
	err = db.Dial()
	if err != nil {
		panic(err)
	}
	req, err := http.NewRequest("HEAD", target, nil)
	if err != nil {
		panic(err)
	}
	tr := &http.Transport{}
	resp, err := tr.RoundTrip(req)
	// resp, err := http.Head("http://reedobrien.com/i/")
	// if err != nil {
	// 	panic(err)
	// }
	defer resp.Body.Close()

	if resp.Header.Get("Last-Modified") != "" {
		lastModified, err = time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
		if err != nil {
			panic(err)
		}
	} else {
		lastModified = time.Now()
	}
	// if no length it is an empty string leave the default 0 assignment
	// in place. Ignore the error because it is ""
	length := resp.Header.Get("Content-Length")
	if length == "" {
		// 0 initialization is fine
	} else {
		l, err := strconv.Atoi(length)
		if err != nil {
			panic(err)
		} else {
			contentLength = int64(l)
		}
	}

	fmt.Println(resp)
	ui := UrlInfo{
		Content_Type:   resp.Header.Get("Content-Type"),
		Content_Length: contentLength,
		Etag:           resp.Header.Get("ETag"),
		Mtime:          lastModified,
		Path:           urlinfo.Path,
		Status:         resp.Status,
		Status_Code:    resp.StatusCode,
	}
	if resp.StatusCode == 302 {
		ui.Location = resp.Header.Get("Location")
	}
	resources := db.ResourceCollection()
	//err = c.Insert(resp)
	err = resources.Insert(&ui)
	if err != nil {
		panic(err)
	}
}
