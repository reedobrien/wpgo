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
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"wp/sss"
)

const concurrency = 100

var meta = map[string][]string{}
var node string
var bucket = flag.StringP("bucket", "b", "", "Use the named bucket")
var prefix = flag.StringP("prefix", "x", "v-", "Set a prefix on the bucketname")
var public = flag.BoolP("public", "p", false, "Makes the uploaded files publicly visible")
var force = flag.BoolP("force", "f", false, "Force upload regardless of existance or mtime")

// XXX: Should this always be true? I think so.
var newer = flag.BoolP("newer", "n", false, "Upload if file time is newer than Last-modified. Default: false")
var newermetamtime = flag.BoolP("newwermetamtime", "N", false, "Upload if file is newer than x-amz-meta-last-modified. Default: false")

// var recursive = flag.BoolP("recursive", "r", false, "Upload everything resursively from the path")
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
	if *bucket == "" {
		bucketname = *prefix + directory
	} else {
		bucketname = *prefix + *bucket
	}
	fmt.Println("Uploading to bucket named: ", bucketname)
	fmt.Println("Publicly visible:", *public)
	s3bucket := sss.GetBucket(sss.Auth(), sss.Region, bucketname) // Should fold these into an options map/struct
	err := filepath.Walk(directory, makeVisitor(uploads, s3bucket, waiter, *public, *force, *newer, *newermetamtime))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	waiter.Wait()
	// fmt.Printf("filepatxh.Walk() returned %v\n", err)
}

func makeVisitor(uploads chan FileUpload, bucket *s3.Bucket, waiter *sync.WaitGroup, public, force, newer, newermetamtime bool) func(string, os.FileInfo, error) error {
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
				uploadFile(fu, public, force, newer, newermetamtime, nil)
			} else {
				waiter.Add(1)
				go uploadFile(fu, public, force, newer, newermetamtime,
					func() {
						waiter.Done()
					})
			}
		}
		return nil
	}
}

func uploadFile(fu FileUpload, public, force, newer, newermetamtime bool, done func()) error {
	if done != nil {
		defer done()
	}
	acl := s3.Private
	if public {
		acl = s3.PublicRead
	}
	fh, err := os.Open(fu.Path)
	if err != nil {
		return err
	}
	defer fh.Close()
	fi, err := fh.Stat()
	if err != nil {
		return err
	}
	remotePath := fu.Path[strings.Index(fu.Path, "/")+1:]
	meta = map[string][]string{
		"last-modified": {fi.ModTime().Format(time.RFC1123)},
	}
	if force {
		if err := fu.Bucket.PutReaderWithMeta(remotePath, fh, fi.Size(), fu.ContentType, acl, meta); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			// os.Exit(1)
		} else {
			fmt.Println("Force uploaded:", remotePath, "Size:", fi.Size(), "content-type:", fu.ContentType)
		}
		return err
	}
	resp, err := fu.Bucket.Head(remotePath, nil)

	if err != nil {
		if e, ok := err.(*s3.Error); ok && e.StatusCode == 404 {
			if err := fu.Bucket.PutReaderWithMeta(remotePath, fh, fi.Size(), fu.ContentType, acl, meta); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				// os.Exit(1)
			} else {
				fmt.Println("Uploaded:", remotePath, "Size:", fi.Size(), "content-type:", fu.ContentType)
			}
		}
	} else {
		if shouldUpdate(resp, fi, newer, newermetamtime) {
			if err := fu.Bucket.PutReaderWithMeta(remotePath, fh, fi.Size(), fu.ContentType, acl, meta); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
			} else {
				// os.Exit(1)
				fmt.Println("Uploaded:", remotePath, "Size:", fi.Size(), "content-type:", fu.ContentType)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Already uploaded:", remotePath, "Last modified:", resp.Header.Get("Last-Modified"))
		}
		return err
	}
	return nil
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

func shouldUpdate(resp *http.Response, fi os.FileInfo, newer, newermetamtime bool) bool {
	var stale bool = false
	filemtime := fi.ModTime()
	if newermetamtime {
		parsed, err := time.Parse(time.RFC1123, resp.Header.Get("x-amz-meta-last-modified"))
		if err != nil {
			// can't see metamtime upload anyhow
			stale = true
			fmt.Fprint(os.Stderr, "Can't read metamtime, setting stale to upload again\n")
		}
		if parsed.Before(filemtime) {
			stale = true
		}
	}
	if newer {
		parsed, err := time.Parse(time.RFC1123, resp.Header.Get("Last-Modified"))
		if err != nil {
			// can't see metamtime upload anyhow
			stale = true
			fmt.Fprint(os.Stderr, "Can't read metamtime, setting stale to upload again\n")
		}
		if parsed.Before(filemtime) {
			stale = true
		}
	}
	return stale
}
