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

func (file *Media) Crop() error {
	img, err := NewNRGBA(file)
	if err != nil {
		return err
	}

	cont, err := img.Crop().GetBytes()
	if err != nil {
		return err
	}

	file.Content = cont
	return nil
}

func (file *Media) AddMask() error {
	img, err := NewNRGBA(file)
	if err != nil {
		return err
	}

	cont, err := img.AddMask().GetBytes()
	if err != nil {
		return err
	}

	file.Content = cont
	return nil
}

func (file *Media) DrawNoise() error {
	img, err := NewNRGBA(file)
	if err != nil {
		return err
	}

	cont, err := img.DrawNoise().GetBytes()
	if err != nil {
		return err
	}

	file.Content = cont
	return nil
}

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

func RandomColor(alpha uint8) color.RGBA {
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
		A: alpha,
	}
}

func (img *ImageNRGBA) AddMask() *ImageNRGBA {
	col := RandomColor(96)
	nm := image.NewNRGBA(image.Rect(0, 0, img.width, img.height))

	for i := 0; i < img.width; i++ {
		for j := 0; j < img.height; j++ {
			nm.Set(i, j, col)
		}
	}
	b := nm.Bounds()

	draw.Draw(img.Image, img.Image.Bounds(), nm, b.Min, draw.Over)

	return img
}

func (img *ImageNRGBA) DrawNoise() *ImageNRGBA {
	density := rand.Intn(10) + 1
	maxSize := (img.height * img.width) / density

	for i := 0; i < maxSize; i++ {

		rw := rand.Intn(img.width)
		rh := rand.Intn(img.height)

		img.Image.Set(rw, rh, RandomColor(255))
		size := rand.Intn(maxSize)
		if size%3 == 0 {
			img.Image.Set(rw+1, rh+1, RandomColor(255))
		}
	}
	return img
}

func (img *ImageNRGBA) Crop() *ImageNRGBA {
	x1 := rand.Intn(img.width / 4)
	y1 := rand.Intn(img.height / 4)
	x2 := (img.width/4)*3 + rand.Intn(img.width/4)
	y2 := (img.height/4)*3 + rand.Intn(img.height/4)
	img.Image = img.Image.SubImage(image.Rect(
		x1,
		y1,
		x2,
		y2,
	)).(*image.NRGBA)

	return img
}
