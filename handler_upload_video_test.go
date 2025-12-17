package main

import (
	"testing"
)

func TestGetVideoAspectRatio_horizontal(t *testing.T) {
	filePath := "./samples/boots-video-horizontal.mp4"
	aspectRatio, err := getVideoAspectRatio(filePath)
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if aspectRatio != "landscape" {
		t.Fatalf("want: 16:9, got: %v\n", aspectRatio)
	}
}

func TestGetVideoAspectRatio_vertical(t *testing.T) {
	filePath := "./samples/boots-video-vertical.mp4"
	aspectRatio, err := getVideoAspectRatio(filePath)
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if aspectRatio != "portrait" {
		t.Fatalf("want: 9:16, got: %v\n", aspectRatio)
	}
}

func TestGetVideoAspectRatio_other(t *testing.T) {
	filePath := "./samples/is-bootdev-for-you.pdf"
	aspectRatio, err := getVideoAspectRatio(filePath)
	if err != nil {
		t.Errorf("err: %v", err)
	}
	if aspectRatio != "other" {
		t.Fatalf("want: other, got: %v\n", aspectRatio)
	}
}

// func TestClassifyAspectRatio(t *testing.T) {
// 	width := 1920
// 	height := 1080
//
// 	aspectRatio := classifyAspectRatio(width, height)
// 	if aspectRatio != "16:9" {
// 		t.Fatalf("want: 16:9, got %v\n", aspectRatio)
// 	}
// }
