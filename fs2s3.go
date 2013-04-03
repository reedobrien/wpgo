/* fs2s3
 */

package main

import (
	"crypto/md5"
	"fmt"
	flag "github.com/ogier/pflag"
	"io/ioutil"
	"launchpad.net/goamz/s3"
	"mime"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	//"wp/sss"
)

const concurrency = 1000

var node string
var bucket = flag.StringP("bucket", "b", "", "Use the named bucket")
var prefix = flag.StringP("prefix", "x", "v-", "Set a prefix on the bucketname")
var public = flag.BoolP("public", "p", false, "Makes the uploaded files publicly visible")
var recursive = flag.BoolP("recursive", "r", false, "Upload everything resursively from the path")

// verify sums?
// guess mimetypes
// force upload even if file is older than uploaded version?
// var encrypted = .... set  x-amz-server-side-encryption: true header

type FileUpload struct {
	ContentType string
	Path        string
	Bucket      *s3.Bucket
}

func main() {
	flag.Parse()
	var bucketname string

	uploads := make(chan FileUpload, 1)
	waiter := &sync.WaitGroup{}
	directory := flag.Arg(0)

	fmt.Println("Bucket : %s", *bucket)
	if *bucket == "" {
		bucketname = *prefix + directory
	} else {
		bucketname = *prefix + *bucket
	}
	fmt.Println("Bucket Name: ", bucketname)
	s3bucket := sss.GetBucket(sss.Auth(), sss.Region, bucketname)
	err := filepath.Walk(directory, makeVisitor(uploads, s3bucket, waiter, *public))
	waiter.Wait()
	fmt.Printf("filepath.Walk() returned %v\n", err)
}

func makeVisitor(uploads chan FileUpload, bucket *s3.Bucket, waiter *sync.WaitGroup, public *bool) func(string, os.FileInfo, error) error {
	return func(fpath string, f os.FileInfo, err error) error {
		node := isfile(f)
		if node {
			contType := mime.TypeByExtension(path.Ext(fpath))
			if contType == "" {
				contType = "binary/octet-stream"
			}
			fu := FileUpload{
				ContentType: contType,
				Path:        fpath,
				Bucket:      bucket,
			}
			if runtime.NumGoroutine() > concurrency {
				uploadFile(fpath, f, *public, nil)
			} else {
				waiter.Add(1)
				go uploadFile(fpath, f, *public,
					func() {
						waiter.Done()
					})
			}
			// create an uploadfiles struct and stick it in the channel
			fmt.Printf("%s\nVisited file:%t\n md5: %x:\n Name: %s\n Content Type: %s\n\n",
				fpath, node, md5sum(fpath), f.Name(), contType)
		}
		return nil
	}
}

func uploadFile(fpath string, f os.FileInfo, bucket *s3.Bucket, public *bool, done func()) error {
	if done != nil {
		defer done()
	}
	remotePath := fpath[strings.Index(fpath, bucket.Name)-len(*bucket.Name)]
	acl := s3.Private
	if *public {
		acl = s3.PublicRead
	}
	fh, err := os.Open(fpath)
	if err != nil {
		return err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return err
	}
	return bucket.PutReader(f.Name(), fh, fi.Size(), contType, acl)
}

//func (b *s3.Bucket) ifModifiedSince(path string, headers)

func md5sum(path string) []byte {
	if data, err := ioutil.ReadFile(path); err == nil {
		h := md5.New()
		h.Write(data)
		return h.Sum(nil)
	}
	return nil
}

func isfile(f os.FileInfo) bool {
	if f.IsDir() {
		return false
	}
	return true
}
