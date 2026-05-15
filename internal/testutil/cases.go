package testutil

import (
	_ "embed"
	"encoding/json"
)

//go:embed cases.json
var casesJSON []byte

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
		DiffColor    *[3]int  `json:"diffColor"`
		DiffColorAlt *[3]int  `json:"diffColorAlt"`
		AAColor      *[3]int  `json:"aaColor"`
	} `json:"opts"`
	Img1 string `json:"img1"`
	Img2 string `json:"img2"`
	Diff string `json:"diff"`
	N    int    `json:"n"`
}

func GetTestCases() ([]RefCase, error) {
	var cases []RefCase
	if err := json.Unmarshal(casesJSON, &cases); err != nil {
		return nil, err
	}
	return cases, nil
}
