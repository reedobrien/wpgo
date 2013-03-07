package db

import (
	"labix.org/v2/mgo"
	"labix.org/v2/mgo/bson"
	"log"
	"wp/fetcher"
)

type Connection struct {
	Session *mgo.Session  "session"
	Db      *mgo.Database "db"
	Url     string        "url"
	Host    string        "host"
	Port    int           "port"
}

type Url struct {
	Id  bson.ObjectId "_id"
	Url string        "url"
}

var current = new(Connection)

func Collection(name string) *mgo.Collection {
	return current.Db.C(name)
}

func UrlCollection() *mgo.Collection {
	return current.Db.C("urls")
}

func ResourceCollection() *mgo.Collection {
	return current.Db.C("resources")
}

func AllUrls() *mgo.Iter {
	c := UrlCollection()
	return c.Find(bson.M{}).Iter()
}

func Seen(url string) bool {
	c := ResourceCollection()
	result := fetcher.UrlInfo{}
	err := c.Find(bson.M{"url": url}).One(&result)
	if err == mgo.ErrNotFound {
		//log.Printf("err: %v", err)
		return false
	} else {
		log.Printf("Problem fetching: %v", err)
	}
	//log.Printf("result: %v", result)
	return true

}

func Dial() error {
	conn_string := "localhost"

	session, err := mgo.Dial(conn_string)

	if err != nil {
		return err
	}
	// Get 10K docs at a time. The default prefetch setup will get
	// a fresh batch at < 2,500.
	session.SetBatch(10000)

	current.Session = session
	current.Db = session.DB("wp")

	index := mgo.Index{
		Key:        []string{"url"},
		Unique:     true,
		DropDups:   true,
		Background: true,
		Sparse:     true,
	}
	// Setup some indexes
	c := session.DB("wp").C("urls")
	err = c.EnsureIndex(index)
	if err != nil {
		return err
	}

	r := session.DB("wp").C("resources")
	indexKeys := []string{"url", "path", "content-type"}
	for _, k := range indexKeys {
		index = mgo.Index{
			Key:        []string{k},
			Unique:     false,
			Background: true,
			Sparse:     true,
		}
		err = r.EnsureIndex(index)
		if err != nil {
			return err
		}
	}
	return nil
}
