package main

import (
	"fmt"
	"log"
	"sync"
	"wp/db"
	"wp/fetcher"
)

var ui fetcher.UrlInfo

func main() {
	log.Println("Starting")
	runtime.GOMAXPROCS(runtime.NumCPU())

	var wg sync.WaitGroup

	concurrency := 1000
	jobs := make(chan fetcher.UrlInfo, concurrency*3)
	err := db.Dial()
	if err != nil {
		panic(err)
	}
	urls := db.UrlCollection()
	uc, _ := urls.Count()
	resources := db.ResourceCollection()
	allurls := db.AllUrls()
	result := db.Url{}
	for i := 0; i < concurrency; i++ {
		wg.Add(concurrency)
		go func() {
			for job := range jobs {
				fmt.Printf("Processed: %s %s\n", job.Path, job.Status)
				err = resources.Insert(&job)
				if err != nil {
					log.Panic(err)
				}
				wg.Done()
			}
		}()
	}
	for allurls.Next(&result) {
		jobs <- fetcher.Head(result.Url)
	}
	wg.Wait()
	log.Println(uc)
	log.Println("Finished")
}
