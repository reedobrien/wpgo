/* fs2s3
 */

package main

import (
	"crypto/md5"
	"fmt"
	flag "github.com/ogier/pflag"
	"io/ioutil"
	//"launchpad.net/goamz/s3"
	"mime"
	"os"
	"path"
	"path/filepath"
	//"wp/sss"
)

var node string
var bucket = flag.StringP("bucket", "b", "", "Use the named bucket")
var prefix = flag.StringP("prefix", "x", "v-", "Set a prefix on the bucketname")
var public = flag.BoolP("public", "p", false, "Makes the uploaded files publicly visible")
var recursive = flag.BoolP("recursive", "r", false, "Upload everything resursively from the path")

// verify sums?
// guess mimetypes
// force upload even if file is older than uploaded version?
// var encrypted = .... set  x-amz-server-side-encryption: true header

func main() {
	var bucketname string
	flag.Parse()
	directory := flag.Arg(0)
	fmt.Println("Bucket : %s", *bucket)
	if *bucket == "" {
		bucketname = *prefix + directory
	} else {
		bucketname = *prefix + *bucket
	}
	fmt.Println("Bucket Name: ", bucketname)
	// open s3 bucket here
	//s3bucket := sss.GetBucket(sss.Auth(), sss.Region, bucketname)
	// headers := map[string][]string{
	// 	"If-Modified-Since": {"Sat, 30 Mar 2012 15:48:13 GMT"},
	// }
	// err = s3bucket.IfModifiedSince("mp4/AirSTAR.mp4", headers)
	// if err != nil {
	// 	fmt.Println(err)
	// }
	//os.Exit(0)
	err := filepath.Walk(directory, visit)
	// if error push donesemaphore down the chan.
	fmt.Printf("filepath.Walk() returned %v\n", err)
}

func visit(fpath string, f os.FileInfo, err error) error {
	node := isfile(f)
	if node {
		contType := mime.TypeByExtension(path.Ext(fpath))
		// create an uploadfiles struct and stick it in the channel
		fmt.Printf("%s\nVisited file:%t\n md5: %x:\n Name: %s\n Content Type: %s\n\n",
			fpath, node, md5sum(fpath), f.Name(), contType)
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
