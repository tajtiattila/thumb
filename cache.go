package thumb

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/tajtiattila/basedir"
)

// Cache is a caching thumbnail generator.
type Cache struct {
	// Errh is called for non-fatal caching errors.
	// The default is to log such errors.
	Errh func(error)

	db *leveldb.DB
}

func OpenCache(path string) (*Cache, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	return &Cache{
		db: db,
		Errh: func(e error) {
			log.Println(e)
		},
	}, nil
}

func OpenDefaultCache(l *log.Logger) (*Cache, error) {
	cacheDir, err := basedir.Cache.EnsureDir("thumbs.leveldb", 0777)
	if err != nil {
		return nil, err
	}

	return OpenCache(filepath.Join(cacheDir))
}

func (c *Cache) Close() error {
	return c.db.Close()
}

func (c *Cache) File(path string, maxw, maxh int) Thumb {
	k := []byte(key(path, maxw, maxh))
	p, err := c.db.Get(k, nil)
	if err == nil {
		return &thumb{raw: p}
	}
	if err != leveldb.ErrNotFound {
		c.Errh(err)
	}

	t := File(path, maxw, maxh)
	p, err = t.JpegBytes()
	if err == nil {
		err = c.db.Put(k, p, nil)
		if err != nil {
			c.Errh(err)
		}
	}

	return t
}

func key(path string, maxw, maxh int) string {
	a, err := filepath.Abs(path)
	if err != nil {
		a = "rel:" + path
	}
	return fmt.Sprintf("%s|%d|%d", a, maxw, maxh)
}
