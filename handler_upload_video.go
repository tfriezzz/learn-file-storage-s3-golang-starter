package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"os/exec"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxMemory = 10 << 30
	r.Body = http.MaxBytesReader(w, r.Body, maxMemory)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "videoID doesn't exist", err)
		return
	}

	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "wrong user", err)
		return
	}

	data, headers, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "can't parse video file", err)
		return
	}
	defer data.Close()

	mediaType := headers.Header.Get("Content-Type")
	mimeType, _, err := mime.ParseMediaType(mediaType)
	if mimeType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "wrong file format", err)
		return
	}

	tempName := "tubely-upload.mp4"
	file, err := os.CreateTemp("", tempName)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "can't make temp file", err)
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()
	bytesCopied, err := io.Copy(file, data)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "io.Copy returned err:", err)
		return
	}
	fmt.Printf("copied %d bytes\n", bytesCopied)

	// filePath := fmt.Sprintf("./%v", tempName)
	prefix, err := getVideoAspectRatio(file.Name())
	if err != nil {
		fmt.Printf("aspectRatio: %v", prefix)
		respondWithError(w, http.StatusInternalServerError, "couldn't get aspectRatio", err)
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't reset file pointer", err)
		return
	}

	key := make([]byte, 32)
	rand.Read(key)
	strKey := hex.EncodeToString(key)
	fileKey := fmt.Sprintf("%s/%s.mp4", prefix, strKey)

	objectParams := &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileKey,
		Body:        file,
		ContentType: &mimeType,
	}

	if _, err := cfg.s3Client.PutObject(r.Context(), objectParams); err != nil {
		respondWithError(w, http.StatusInternalServerError, "writesomethinghere", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "somethingelse", err)
	}

	videoURL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, fileKey)
	fmt.Println(videoURL)
	dbVideo.VideoURL = &videoURL

	if err := cfg.db.UpdateVideo(dbVideo); err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update videoURL", err)
		return
	}
}

func getVideoAspectRatio(filePath string) (string, error) {
	fmt.Printf("filePath: %v\n", filePath)
	// arg := fmt.Sprintf("-v error -print_format json -show_streams %v", filePath)
	// args := []string{
	// "-v", "error", "-print_format", "json", "-show_streams", filePath,
	// }
	// cmd := exec.Command("ffprobe", strings.Join(args, " "))
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	// b := bytes.NewBuffer(make([]byte, 20))

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		log.Printf("ffprobe err: %v", err)
		// return "other", nil
	}

	var output struct {
		Streams []struct {
			Width  int `json:"width,omitempty"`
			Height int `json:"height,omitempty"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return "", fmt.Errorf("unmarshal failed: %v", err)
	}

	if len(output.Streams) == 0 {
		return "", fmt.Errorf("no video streams found")
	}

	width := output.Streams[0].Width
	height := output.Streams[0].Height
	// aspectRatio := classifyAspectRatio(width, height)
	// fmt.Printf("file: %v, aspectRatio: %v\n", filePath, aspectRatio)

	if width > height {
		return "landscape", nil
	} else if height > width {
		return "portrait", nil
		// } else if height == 0 || width == 0 || height == width {
	} else {
		return "other", nil
	}

	// switch aspectRatio {
	// case "16:9":
	// 	return "16:9", nil
	// case "9:16":
	// 	return "9:16", nil
	// default:
	// 	return "other", nil
	// }
}

// func classifyAspectRatio(width, height int) string {
// 	tolerance := 0.05
// 	ratio := float64(width) / float64(height)
//
// 	landscape169 := 16.0 / 9.0 // ~1.7778
// 	portrait916 := 9.0 / 16.0  // ~0.5625
//
// 	if math.Abs(ratio-landscape169) < tolerance {
// 		return "landscape 16:9"
// 	}
// 	if math.Abs(ratio-portrait916) < tolerance {
// 		return "portrait 9:16"
// 	}
//
// 	return "other"
// }

// func gcd(a, b int) int {
// 	for b != 0 {
// 		a, b = b, a%b
// 	}
// 	return a
// }
//
// func reducedAspectRatio(width, height int) (int, int) {
// 	divisor := gcd(width, height)
// 	return width / divisor, height / divisor
// }
//
// func aspectRatioString(width int, height int) (aspectRatio string) {
// 	w, h := reducedAspectRatio(width, height)
// 	aspectRatio = fmt.Sprintf("%d:%d", w, h)
// 	return aspectRatio
// }
