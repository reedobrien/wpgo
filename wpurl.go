package main

import (
	"fmt"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"os"
	"runtime"
	"sync"
	"wp/db"
	"wp/fetcher"
	"wp/sss"
)

type Job struct {
	UrlInfo fetcher.UrlInfo
	Body    []byte
}

func main() {
	log.Println("Starting")
	concurrency := runtime.NumCPU()
	runtime.GOMAXPROCS(runtime.NumCPU())
	var wg sync.WaitGroup

	jobs := make(chan Job, 1000)
	err := db.Dial()
	if err != nil {
		panic(err)
	}
	errors := db.ErrorCollection()
	urls := db.UrlCollection()
	uc, _ := urls.Count()
	resources := db.ResourceCollection()
	allurls := db.AllUrls()
	result := db.Url{}
	s3bucket := sss.GetBucket(auth(), sss.Region, sss.BucketName)
	log.Printf("Got bucket: %v\n", s3bucket.Name)
	wg.Add(1)
	for i := 0; i < concurrency; i++ {
		go func() {
			job := <-jobs
			if job.UrlInfo.Status_Code == 200 {
				err = s3bucket.Put(
					job.UrlInfo.Path, job.Body, job.UrlInfo.Content_Type, s3.PublicRead)
				if err != nil {
					log.Println("***********************************************************")
					log.Printf("Failed to put file for: %s\nError%v\n", job.UrlInfo.Url, err)
					log.Printf("Path: %s\nSize:%d\n", job.UrlInfo.Path, job.UrlInfo.Content_Length)
					log.Println("***********************************************************")
					//log.Printf("JOB %v\n", job.Body)
					errors.Insert(&job.UrlInfo)
				} else {
					err = resources.Insert(&job.UrlInfo)
					if err != nil {
						fmt.Printf("%s\n", err)
					}
				}
			} else {
				err = resources.Insert(&job.UrlInfo)
				if err != nil {
					fmt.Printf("%s\n", err)
				}
			}
			wg.Done()

		}()
	}

	go func() {
		for allurls.Next(&result) {
			seen := db.Seen(result.Url)
			if seen == false {
				job := Job{}
				j, r := fetcher.Get(result.Url)
				job.UrlInfo = j
				job.Body = r
				jobs <- job
				wg.Add(1)
			}
		}
		wg.Done()
	}()

	wg.Wait()
	log.Println(uc, wg)
	log.Println("Finished")
}

func auth() aws.Auth {
	creds, err := aws.EnvAuth()
	if err != nil {
		log.Println("Error with aws credentials: %v", err)
		os.Exit(1)
	}
	return creds
}
