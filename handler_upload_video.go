package main

import (
	"bytes"
	"context"
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
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/database"
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

	processedFilePath, err := processVideoForFastStart(file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "can't process video", err)
		return
	}

	prefix, err := getVideoAspectRatio(file.Name())
	if err != nil {
		fmt.Printf("aspectRatio: %v", prefix)
		respondWithError(w, http.StatusInternalServerError, "couldn't get aspectRatio", err)
	}

	processedFile, err := os.Open(processedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "can't open processed file", err)
		return
	}
	defer os.Remove(processedFilePath)
	defer processedFile.Close()

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
		Body:        processedFile,
		ContentType: &mimeType,
	}

	if _, err := cfg.s3Client.PutObject(r.Context(), objectParams); err != nil {
		respondWithError(w, http.StatusInternalServerError, "writesomethinghere", err)
		return
	}

	dbVideo, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "somethingelse", err)
		return
	}

	// e videoURL := fmt.Sprintf("https://%v.s3.%v.amazonaws.com/%v", cfg.s3Bucket, cfg.s3Region, fileKey)

	// HACK: Is this correct?
	videoURL := fmt.Sprintf("%v,%v", cfg.s3Bucket, fileKey)
	fmt.Println(videoURL)

	dbVideo.VideoURL = &videoURL

	signedVideo, err := cfg.dbVideoToSignedVideo(dbVideo)
	if err != nil {
	}

	// if err := cfg.db.UpdateVideo(dbVideo); err != nil {
	if err := cfg.db.UpdateVideo(signedVideo); err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't update videoURL", err)
		return
	}
}

func getVideoAspectRatio(filePath string) (string, error) {
	fmt.Printf("filePath: %v\n", filePath)
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		log.Printf("ffprobe err: %v", err)
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

	if width > height {
		return "landscape", nil
	} else if height > width {
		return "portrait", nil
		// } else if height == 0 || width == 0 || height == width {
	} else {
		return "other", nil
	}
}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := fmt.Sprintf("%v.processing", filePath)
	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return outputFilePath, nil
}

func generatePresignedURL(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
	client := s3.NewPresignClient(s3Client)

	getObjectArgs := s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	}

	res, err := client.PresignGetObject(context.Background(), &getObjectArgs, s3.WithPresignExpires(expireTime))
	if err != nil {
		return "", fmt.Errorf("PresignGetObject err: %v", err)
	}

	fmt.Printf("request: %v", res.URL)
	return res.URL, nil
}

func (cfg *apiConfig) dbVideoToSignedVideo(video database.Video) (database.Video, error) {
	fmt.Printf("before VideoURL: %v", video.VideoURL)

	videoURL := video.VideoURL

	if videoURL == nil {
		fmt.Println("empty video address")
		return video, nil
	}
	if len(*videoURL) == 0 {
		fmt.Println("empty videoURL")
		return video, nil
	}
	if ok := strings.Contains(*videoURL, ","); !ok {
		fmt.Println("no comma")
		return video, nil
	}

	bucketKey := strings.Split(*videoURL, ",")
	bucket := bucketKey[0]
	key := bucketKey[1]

	presignedURL, err := generatePresignedURL(cfg.s3Client, bucket, key, time.Hour) // HACK:time.Duration?
	if err != nil {
		return database.Video{}, fmt.Errorf("generatePresignedURL err: %v", err)
	}

	video.VideoURL = &presignedURL
	fmt.Printf("after VideoURL: %v", video.VideoURL)

	return video, nil
}
