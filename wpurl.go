package main

import (
	//"fmt"
	"wp/db"
	"wp/fetcher"
)

func main() {
	target := "http://www.nasa.gov/topics/nasalife/features/worldbook.html"
	// target = "http://www.nasa.gov/worldbook/index.html"
	/// get the collection connection
	err := db.Dial()
	if err != nil {
		panic(err)
	}
	ui := fetcher.Fetch(target)
	resources := db.ResourceCollection()
	//err = c.Insert(resp)
	err = resources.Insert(&ui)
	if err != nil {
		panic(err)
	}
}
