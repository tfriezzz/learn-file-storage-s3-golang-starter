package main

import (
	"crypto/rand"
	b64 "encoding/base64"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here
	const maxMemory = 10 << 20
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		log.Printf("ParseMultipartForm returned err: %v", err)
	}

	data, headers, err := r.FormFile("thumbnail")
	if err != nil {
		log.Printf("FormFile returned err: %v", err)
	}

	mediaType := headers.Header.Get("Content-Type")
	mimeType, params, err := mime.ParseMediaType(mediaType)
	if err != nil {
		log.Printf("ParseMediaType returned err: %v", err)
	}

	fmt.Printf("mediaType: %v, mimeType: %v\nparams: %v\n", mediaType, mimeType, params)
	if mimeType != "image/jpeg" && mimeType != "image/png" {
		respondWithError(w, http.StatusBadRequest, "unsupported file format", nil)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		log.Printf("GetVideo returned err: %v", err)
	}

	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		fmt.Printf("expected: %v\ngot: %v\n", userID, video.UserID)
		return
	}

	splitString := strings.Split(mediaType, "/")
	fileExtension := splitString[1]

	key := make([]byte, 32)
	randByte, _ := rand.Read(key)
	fmt.Printf("Read returned v% bytes", randByte)
	randString := b64.RawURLEncoding.EncodeToString(key)

	filePath := fmt.Sprintf("%v.%v", randString, fileExtension)
	assetsRoot := cfg.assetsRoot
	completePath := filepath.Join(assetsRoot, filePath)

	newFile, err := os.Create(completePath)
	if err != nil {
		log.Printf("os.Create returned err: %v", err)
	}

	bytesWritten, err := io.Copy(newFile, data)
	if err != nil {
		log.Printf("io.Copy returned err: %v", err)
	}
	fmt.Printf("copied %d bytes\n", bytesWritten)

	thumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v", cfg.port, filePath)
	fmt.Printf("thumbnailURL: %v", thumbnailURL)

	video.ThumbnailURL = &thumbnailURL
	if err := cfg.db.UpdateVideo(video); err != nil {
		fmt.Printf("UpdateVideo returned err: %v", err)
	}

	respondWithJSON(w, http.StatusOK, video)
}
