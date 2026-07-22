package testutil

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	"image/png"
)

//go:embed cases.json
var casesJSON []byte

//go:embed data/cases/*.png
var casesFS embed.FS

type RefCase struct {
	Name string `json:"name"`
	W    int    `json:"w"`
	H    int    `json:"h"`
	Opts struct {
		Threshold    *float64 `json:"threshold"`
		IncludeAA    *bool    `json:"includeAA"`
		Alpha        *float64 `json:"alpha"`
		DiffMask     *bool    `json:"diffMask"`
		Checkerboard *bool    `json:"checkerboard"`
		WindowSize   *int     `json:"windowSize"`
		DiffColor    *[3]int  `json:"diffColor"`
		DiffColorAlt *[3]int  `json:"diffColorAlt"`
		AAColor      *[3]int  `json:"aaColor"`
	} `json:"opts"`
	N int `json:"n"`

	// Populated by GetTestCases from the sibling PNG files.
	Img1 []byte `json:"-"`
	Img2 []byte `json:"-"`
	Diff []byte `json:"-"`
}

func GetTestCases() ([]RefCase, error) {
	var cases []RefCase
	if err := json.Unmarshal(casesJSON, &cases); err != nil {
		return nil, err
	}
	for i := range cases {
		c := &cases[i]
		for _, part := range []struct {
			suffix string
			dst    *[]byte
		}{{"a", &c.Img1}, {"b", &c.Img2}, {"diff", &c.Diff}} {
			pix, err := loadCasePNG(c.Name+"-"+part.suffix+".png", c.W, c.H)
			if err != nil {
				return nil, fmt.Errorf("case %q %s: %w", c.Name, part.suffix, err)
			}
			*part.dst = pix
		}
	}
	return cases, nil
}

func loadCasePNG(name string, w, h int) ([]byte, error) {
	data, err := casesFS.ReadFile("data/cases/" + name)
	if err != nil {
		return nil, err
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		return nil, fmt.Errorf("dimensions %dx%d != %dx%d", img.Bounds().Dx(), img.Bounds().Dy(), w, h)
	}
	if nrgba, ok := img.(*image.NRGBA); ok && nrgba.Stride == w*4 && nrgba.Rect.Min == (image.Point{}) {
		return nrgba.Pix, nil
	}
	dst := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Src)
	return dst.Pix, nil
}
