package sss

import (
	"launchpad.net/goamz/aws"
	"launchpad.net/goamz/s3"
	"log"
	// "os"
)

var BucketName = "v-fetched.nasa.gov"
var Region = aws.USEast

// Get a handle onto an s3 bucket
func GetBucket(creds aws.Auth, region aws.Region, bucket_name string) *s3.Bucket {
	log.Println("Setting up s3 access")
	sss := s3.New(creds, region)
	bucket := sss.Bucket(bucket_name)
	// _, err := bucket.Get("sad;lkfjls;k")
	// Create the bucket if it doesn't exist.
	// if err != nil {
	// 	s3err, _ := err.(*s3.Error)
	// 	if s3err.Code == "NoSuchBucket" {
	// 		log.Println("Creating s3 bucket")
	// 		bucket.PutBucket(s3.Private)
	// 	}
	// }
	return bucket
}
