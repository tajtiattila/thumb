package thumb

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"sync"

	"github.com/tajtiattila/metadata"
	"github.com/tajtiattila/metadata/orient"

	"golang.org/x/image/draw"
)

var magick struct {
	checked bool
	path    string
	broken  bool
}

func hasMagick() bool {
	return false

	if !magick.checked {
		magick.checked = true
		mp := os.Getenv("MAGICK_PATH")
		if mp != "" {
			magick.path = mp
			return true
		}

		mp, err := exec.LookPath("magick")
		if err == nil {
			magick.path = mp
			return true
		}

		log.Println("magick unavailable")
		magick.broken = true
	}

	return !magick.broken
}

// File returns the thumb of the file.
func File(path string, maxw, maxh int) Thumb {
	if hasMagick() {
		r, err := magickThumb(path, maxw, maxh)
		if err == nil {
			return r
		}
	}

	return pureThumb(path, maxw, maxh)
}

func FromReader(r io.Reader, maxw, maxh int) Thumb {
	raw, err := ioutil.ReadAll(r)
	if err != nil {
		return errThumb(err)
	}

	return pureThumbBytes(raw, maxw, maxh)
}

func pureThumb(fn string, maxw, maxh int) Thumb {
	raw, err := ioutil.ReadFile(fn)
	if err != nil {
		return errThumb(err)
	}

	return pureThumbBytes(raw, maxw, maxh)
}

func pureThumbBytes(raw []byte, maxw, maxh int) Thumb {
	im, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return errThumb(err)
	}

	m, err := metadata.Parse(bytes.NewReader(raw))
	if err != nil && err != metadata.ErrUnknownFormat {
		log.Println("metadata:", err)
	}

	var w, h int
	if m != nil && orient.IsTranspose(m.Orientation) {
		// image is going to be transposed,
		// swap target width/height
		w, h = thumbSize(im, maxh, maxw)
	} else {
		w, h = thumbSize(im, maxw, maxh)
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))

	draw.Draw(dst, dst.Bounds(), image.White, image.ZP, draw.Src)
	draw.BiLinear.Scale(dst, dst.Bounds(), im, im.Bounds(), draw.Src, nil)

	var t image.Image = dst
	if m != nil {
		t = orient.Orient(t, m.Orientation)
	}

	return &thumb{im: t}
}

func thumbSize(im image.Image, maxw, maxh int) (w, h int) {
	s := im.Bounds().Size()
	if s.X <= maxw && s.Y <= maxh {
		return s.X, s.Y
	}
	w = s.X * maxh / s.Y
	if w <= maxw {
		return w, maxh
	}
	h = s.Y * maxw / s.X
	return maxw, h
}

func magickThumb(fn string, maxw, maxh int) (Thumb, error) {
	cmd := exec.Command(magick.path, "convert",
		"-auto-orient",
		"-thumbnail", fmt.Sprintf("%dx%d", maxw, maxh),
		"jpg:-")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		log.Println("magick failed with", err)
		magick.broken = true
		return nil, err
	}
	buf, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}
	return &thumb{raw: buf}, nil
}

type Thumb interface {
	JpegBytes() ([]byte, error)
	Image() (image.Image, error)
}

type errorThumb struct {
	err error
}

func errThumb(err error) Thumb { return &errorThumb{err: err} }

func (t *errorThumb) JpegBytes() ([]byte, error)  { return nil, t.err }
func (t *errorThumb) Image() (image.Image, error) { return nil, t.err }

type thumb struct {
	mu  sync.Mutex
	raw []byte
	im  image.Image
}

func (t *thumb) JpegBytes() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.raw == nil {
		if t.im == nil {
			panic("internal: thumb image is nil")
		}
		b := new(bytes.Buffer)
		err := jpeg.Encode(b, t.im, nil)
		if err != nil {
			return nil, err
		}
		t.raw = b.Bytes()
	}
	return t.raw, nil
}

func (t *thumb) Image() (image.Image, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.im == nil {
		if t.raw == nil {
			panic("internal: thumb image is nil")
		}
		im, _, err := image.Decode(bytes.NewReader(t.raw))
		if err != nil {
			return nil, err
		}
		t.im = im
	}
	return t.im, nil
}
