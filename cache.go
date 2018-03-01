package thumb

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"

	"github.com/tajtiattila/basedir"
	"github.com/tajtiattila/metadata"
)

// Cache is a caching thumbnail generator.
type Cache struct {
	// Errh is called for non-fatal caching errors.
	// The default is to log such errors.
	Errh func(error)

	kv KeyValue
}

// NewCache returns a new Cache that use kv as storage.
func NewCache(kv KeyValue) *Cache {
	return &Cache{
		Errh: func(e error) {
			log.Println(e)
		},
		kv: kv,
	}
}

// OpenDefaultCache returns the default on-disk Cache.
func OpenDefaultCache() (*Cache, error) {
	cacheDir, err := basedir.Cache.EnsureDir("thumbs.leveldb", 0777)
	if err != nil {
		return nil, err
	}

	kv, err := OpenLevelDBKeyValue(cacheDir)
	if err != nil {
		return nil, err
	}

	return NewCache(kv), nil
}

// Close closes the underlying database of c.
func (c *Cache) Close() error {
	return c.kv.Close()
}

// File returns the Thumb of the file using c.
func (c *Cache) File(path string, maxw, maxh int) (*Thumb, error) {
	// make path absolute and clean it
	abs, err := filepath.Abs(path)
	if err != nil {
		c.Errh(err)
		return File(path, maxw, maxh)
	}

	metaKey := "meta|" + abs
	rawMeta, metaErr := c.kv.Get(metaKey)
	if metaErr != nil && metaErr != ErrNotFound {
		c.Errh(metaErr)
	}

	thumb, thumbErr := c.getThumb(abs, maxw, maxh)
	if thumbErr != nil && thumbErr != ErrNotFound {
		c.Errh(thumbErr)
	}

	if metaErr == nil && thumbErr == nil {
		var meta metadata.Metadata
		err := json.Unmarshal(rawMeta, &meta)
		if err == nil {
			thumb.Meta = meta
			return thumb, nil
		}
	}

	thumb, err = File(abs, maxw, maxh)
	if err != nil {
		return nil, err
	}

	err = c.putThumb(abs, maxw, maxh, thumb)
	if err != nil {
		c.Errh(err)
	}

	rawMeta, err = json.Marshal(thumb.Meta)
	if err != nil {
		panic(err)
	}

	if err := c.kv.Put(metaKey, rawMeta); err != nil {
		c.Errh(err)
	}

	return thumb, nil
}

func (c *Cache) getThumb(abs string, maxw, maxh int) (*Thumb, error) {
	raw, err := c.kv.Get(thumbKey(abs, maxw, maxh))
	if err != nil {
		return nil, err
	}

	dx, nx := binary.Uvarint(raw)
	if nx <= 0 || dx > uint64(maxw) {
		return nil, errors.New("invalid dx")
	}
	raw = raw[nx:]

	dy, ny := binary.Uvarint(raw)
	if ny <= 0 || dy > uint64(maxh) {
		return nil, errors.New("invalid dy")
	}
	raw = raw[ny:]

	return &Thumb{
		Jpeg: raw,
		Dx:   int(dx),
		Dy:   int(dy),
	}, nil
}

func (c *Cache) putThumb(abs string, maxw, maxh int, t *Thumb) error {
	sz := make([]byte, 2*binary.MaxVarintLen64)
	nx := binary.PutUvarint(sz, uint64(t.Dx))
	ny := binary.PutUvarint(sz[nx:], uint64(t.Dy))

	raw := new(bytes.Buffer)
	raw.Write(sz[:nx+ny])
	raw.Write(t.Jpeg)

	return c.kv.Put(thumbKey(abs, maxw, maxh), raw.Bytes())
}

func thumbKey(abs string, maxw, maxh int) string {
	return fmt.Sprintf("thumb|%s|%d|%d", abs, maxw, maxh)
}
