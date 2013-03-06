package main

import (
	"log"
	"sync"
	"wp/db"
	"wp/fetcher"
)

var ui fetcher.UrlInfo

func main() {
	// target := "http://www.nasa.gov/topics/nasalife/features/worldbook.html"
	// target = "http://www.nasa.gov/worldbook/index.html"
	/// get the collection connection
	var wg sync.WaitGroup
	concurrency := 10
	jobs := make(chan fetcher.UrlInfo, concurrency)
	err := db.Dial()
	if err != nil {
		panic(err)
	}
	resources := db.ResourceCollection()
	allurls := db.AllUrls()
	result := db.Url{}
	for i := 0; i < concurrency; i++ {
		go func() {
			for job := range jobs {
				log.Printf("Processed: %s %s\n", result.Url, ui.Status)
				err = resources.Insert(&job)
				if err != nil {
					log.Panic(err)
				}
				wg.Done()
			}
		}()
	}
	for allurls.Next(&result) {
		wg.Add(1)
		jobs <- fetcher.Head(result.Url)
	}
	wg.Wait()
	log.Println("Finished")

}
