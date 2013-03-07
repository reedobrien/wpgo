package main

import (
	"fmt"
	"log"
	"runtime"
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
	jobs := make(chan fetcher.UrlInfo, concurrency)
	err := db.Dial()
	if err != nil {
		panic(err)
	}
	urls := db.UrlCollection()
	uc, _ := urls.Count()
	resources := db.ResourceCollection()
	allurls := db.AllUrls()
	result := db.Url{}
	wg.Add(uc)
	for i := 0; i < concurrency; i++ {
		go func() {
			for job := range jobs {
				// fmt.Printf("Processed: %s %s\n", job.Path, job.Status)
				err = resources.Insert(&job)
				if err != nil {
					fmt.Printf("%s\n", err)
					break
				}
				wg.Done()
			}
		}()
	}
	counter := 0
	go func() {
		for allurls.Next(&result) {
			counter += 1
			jobs <- fetcher.Head(result.Url)
		}
	}()
	wg.Wait()
	// log.Println(counter)
	log.Println(uc)
	log.Println("Finished")
}
