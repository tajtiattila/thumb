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

	"github.com/pkg/errors"
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

// File returns the Thumb of the file.
func File(path string, maxw, maxh int) (*Thumb, error) {
	if hasMagick() {
		r, err := magickThumb(path, maxw, maxh)
		if err == nil {
			return r, nil
		}
	}

	return pureThumb(path, maxw, maxh)
}

// FromReader returns the Thumb for r.
func FromReader(r io.Reader, maxw, maxh int) (*Thumb, error) {
	raw, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	return pureThumbBytes(raw, maxw, maxh)
}

func pureThumb(fn string, maxw, maxh int) (*Thumb, error) {
	raw, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	return pureThumbBytes(raw, maxw, maxh)
}

func pureThumbBytes(raw []byte, maxw, maxh int) (*Thumb, error) {
	im, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, err
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

	buf := new(bytes.Buffer)
	if err := jpeg.Encode(buf, t, nil); err != nil {
		return nil, err
	}

	thumb := &Thumb{
		Jpeg: buf.Bytes(),
		Dx:   t.Bounds().Dx(),
		Dy:   t.Bounds().Dy(),
	}
	if m != nil {
		thumb.Meta = *m
	}
	return thumb, nil
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

func magickThumb(fn string, maxw, maxh int) (*Thumb, error) {
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
		return nil, errors.Wrapf(err, "can't start to get thumb of %q", fn)
	}
	raw, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, errors.Wrapf(err, "can't read magick thumb of %q", fn)
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	conf, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.Wrapf(err, "can't decode magick thumb of %q", fn)
	}

	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m, err := metadata.Parse(f)
	if err != nil && err != metadata.ErrUnknownFormat {
		log.Println("metadata:", err)
	}

	thumb := &Thumb{
		Jpeg: raw,
		Dx:   conf.Width,
		Dy:   conf.Height,
	}

	if m != nil {
		thumb.Meta = *m
	}

	return thumb, nil
}

// Thumb holds a JPEG thumbnail and metadata of an image.
type Thumb struct {
	// Raw JPEG thumbnail
	Jpeg []byte

	// Thumbnail dimensions
	Dx, Dy int

	// Exif/XMP metadata of original image
	Meta metadata.Metadata
}
