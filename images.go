package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"math/rand"
)

type ImageNRGBA struct {
	Image   *image.NRGBA
	Decoder func(io.Reader) (image.Image, error)
	Encoder func(io.Writer, image.Image) error

	height int
	width  int
}

func NewNRGBA(file *Media) (*ImageNRGBA, error) {
	nrgba := ImageNRGBA{}

	switch file.Ext {

	case ".png":
		nrgba.Decoder = png.Decode
		nrgba.Encoder = png.Encode

	case ".jpg", ".jpeg":
		nrgba.Decoder = jpeg.Decode
		nrgba.Encoder = func(r io.Writer, c image.Image) error {
			return jpeg.Encode(r, c, nil)
		}

	default:
		return nil, errors.New("unsupported type")
	}

	img, err := nrgba.Decoder(bytes.NewReader(file.Content))
	if err != nil {
		return nil, err
	}

	b := img.Bounds()
	nrgba.height = b.Dy()
	nrgba.width = b.Dx()

	nrgba.Image = image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(nrgba.Image, nrgba.Image.Bounds(), img, b.Min, draw.Src)

	return &nrgba, nil
}

func (img *ImageNRGBA) GetBytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	err := img.Encoder(buf, img.Image)

	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func RandomColor() color.RGBA {
	red := rand.Intn(255)
	green := rand.Intn(255)
	blue := rand.Intn(255)

	if (red + green) > 400 {
		blue = 0
	} else {
		blue = 400 - green - red
	}

	if blue > 255 {
		blue = 255
	}

	return color.RGBA{
		R: uint8(red),
		G: uint8(green),
		B: uint8(blue),
		A: uint8(255),
	}
}

func (img *ImageNRGBA) DrawNoise() *ImageNRGBA {
	density := rand.Intn(10) + 1
	maxSize := (img.height * img.width) / density

	for i := 0; i < maxSize; i++ {

		rw := rand.Intn(img.width)
		rh := rand.Intn(img.height)

		img.Image.Set(rw, rh, RandomColor())
		size := rand.Intn(maxSize)
		if size%3 == 0 {
			img.Image.Set(rw+1, rh+1, RandomColor())
		}
	}
	return img
}
