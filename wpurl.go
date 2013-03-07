package main

import (
	"fmt"
	"io/ioutil"
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	"net/http"
	"os"
	"runtime"
	"sync"
	"wp/db"
	"wp/fetcher"
	"wp/sss"
)

type Job struct {
	UrlInfo fetcher.UrlInfo
	Resp    *http.Response
}

func main() {
	log.Println("Starting")

	runtime.GOMAXPROCS(runtime.NumCPU())
	var wg sync.WaitGroup

	concurrency := 1
	jobs := make(chan Job, concurrency)
	err := db.Dial()
	if err != nil {
		panic(err)
	}
	urls := db.UrlCollection()
	uc, _ := urls.Count()
	resources := db.ResourceCollection()
	allurls := db.AllUrls()
	result := db.Url{}
	s3bucket := sss.GetBucket(auth(), sss.Region, sss.BucketName)
	log.Printf("Got bucket: %v\n", s3bucket)
	wg.Add(uc)
	for i := 0; i < concurrency; i++ {

		go func() {
			for job := range jobs {
				// fmt.Printf("Processed: %s %s\n", job.Path, job.Status)
				err = resources.Insert(&job.UrlInfo)
				if err != nil {
					fmt.Printf("%s\n", err)
					break
				}
				body, err := ioutil.ReadAll(job.Resp.Body)
				if err != nil {
					log.Printf("Failed to do something %v\n", err)
				}
				log.Printf("Uhhh %v\n XXXXX", body)
				err = s3bucket.PutReader(
					sss.BucketName, job.Resp.Body, job.Resp.ContentLength, job.UrlInfo.Content_Type, s3.PublicRead)
				if err != nil {
					log.Printf("Failed to put file for %s: %v\n", job.UrlInfo, err)
					log.Printf("JOB %v\n", job.Resp.Body)
				}
				wg.Done()
			}
		}()
	}
	counter := 0
	go func() {
		for allurls.Next(&result) {
			counter += 1
			seen := db.Seen(result.Url)
			if seen == false {
				job := Job{}
				j, r := fetcher.Get(result.Url)
				job.UrlInfo = j
				job.Resp = r
				jobs <- job
			}
			wg.Done()
		}
	}()
	wg.Wait()
	// log.Println(counter)
	log.Println(uc)
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
