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
const sseKey string = "server-side-encryption"

var sseValue = []string{"AES256"}

var meta = map[string][]string{}
var node string
var bucket = flag.StringP("bucket", "b", "", "Use the named bucket")
var prefix = flag.StringP("prefix", "x", "", "Set upload path prefix. i.e., '-x location/'. NB: Add a slash if you want it!")
var public = flag.BoolP("public", "p", false, "Makes the uploaded files publicly visible")
var force = flag.BoolP("force", "f", false, "Force upload regardless of existance or mtime")
var sse = flag.BoolP("sse", "e", false, "Use server side encryption")
var mimetype = flag.StringP("mimetype", "m", "binary/octet-stream", "Set a fallback/default mimetype string.")

// XXX: Should this always be true? I think so.
var newer = flag.BoolP("newer", "n", false, "Upload if file time is newer than Last-modified")
var newermetamtime = flag.BoolP("newermetamtime", "N", false, "Upload if file is newer than x-amz-meta-last-modified")

// var recursive = flag.BoolP("recursive", "r", false, "Upload everything resursively from the path")
// verify sums?
// add a flag to use text/html as the default/fallback mime type instead of binary/octet-stream

type FileUpload struct {
	ContentType string
	Path        string
	Bucket      *s3.Bucket
}

type args struct {
	force, newer, newermetamtime, public, sse bool
	bucket, mimetype, prefix                  string
}

func main() {
	flag.Parse()
	var bucketname string

	// TODO, remove channel? not using it or waiter
	uploads := make(chan FileUpload, 1)
	waiter := &sync.WaitGroup{}

	directory := flag.Arg(0)

	if *bucket == "" {
		bucketname = directory
	} else {
		bucketname = *bucket
	}

	fmt.Println("Uploading to bucket named: ", bucketname)
	fmt.Println("Publicly visible:", *public)
	s3bucket := sss.GetBucket(sss.Auth(), sss.Region, bucketname)

	err := filepath.Walk(directory, makeVisitor(uploads, s3bucket, waiter, args{
		force:          *force,
		mimetype:       *mimetype,
		newer:          *newer,
		newermetamtime: *newermetamtime,
		prefix:         *prefix,
		public:         *public,
		sse:            *sse,
	}))

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	waiter.Wait()
	// fmt.Printf("filepatxh.Walk() returned %v\n", err)
}

func makeVisitor(uploads chan FileUpload, bucket *s3.Bucket, waiter *sync.WaitGroup, args args) func(string, os.FileInfo, error) error {
	return func(fpath string, f os.FileInfo, err error) error {
		node := isfile(f)
		if node {
			contType := mime.TypeByExtension(path.Ext(fpath))
			if contType == "" {
				contType = args.mimetype
			}
			fu := FileUpload{
				ContentType: contType,
				Path:        fpath,
				Bucket:      bucket,
			}
			if runtime.NumGoroutine() > concurrency {
				uploadFile(fu, args, nil)
			} else {
				waiter.Add(1)
				go uploadFile(fu, args, func() {
					waiter.Done()
				})
			}
		}
		return nil
	}
}

func uploadFile(fu FileUpload, args args, done func()) error {
	if done != nil {
		defer done()
	}
	acl := s3.Private
	if args.public {
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
	options := &s3.Options{SSE: args.sse}
	// TODO: to naive? strings.Join better?
	remotePath := args.prefix + fu.Path[strings.Index(fu.Path, "/")+1:]
	options.Meta = map[string][]string{
		"last-modified": {fi.ModTime().Format(time.RFC1123)},
	}
	if args.force {
		if err := fu.Bucket.PutReader(remotePath, fh, fi.Size(), fu.ContentType, acl, *options); err != nil {
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
			if err := fu.Bucket.PutReader(remotePath, fh, fi.Size(), fu.ContentType, acl, *options); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				// os.Exit(1)
			} else {
				fmt.Println("Uploaded:", remotePath, "Size:", fi.Size(), "content-type:", fu.ContentType)
			}
		}
	} else {
		if shouldUpdate(resp, fi, args.newer, args.newermetamtime) {
			if err := fu.Bucket.PutReader(remotePath, fh, fi.Size(), fu.ContentType, acl, *options); err != nil {
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
