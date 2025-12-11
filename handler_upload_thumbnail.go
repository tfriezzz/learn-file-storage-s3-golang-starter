package main

import (
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
	// dataSlice, err := io.ReadAll(data)
	// if err != nil {
	// 	log.Printf("io.ReadAll returned err: %v", err)
	// }

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		log.Printf("GetVideo returned err: %v", err)
	}
	if userID != video.UserID {
		respondWithError(w, http.StatusUnauthorized, "unauthorized", nil)
		fmt.Printf("expected: %v\ngot: %v\n", userID, video.UserID)
		return
	}
	// thumbnail := thumbnail{
	// 	data:      dataSlice,
	// 	mediaType: mediaType,
	// }
	// base64Data := base64.StdEncoding.EncodeToString(thumbnail.data)

	// videoThumbnails[video.ID] = thumbnail

	splitString := strings.Split(mediaType, "/")
	fileExtension := splitString[1]

	filePath := fmt.Sprintf("%v.%v", videoID, fileExtension)
	// fmt.Printf("thumbnailURL: %v", thumbnailURL)

	assetsRoot := cfg.assetsRoot
	completePath := filepath.Join(assetsRoot, filePath)
	fmt.Printf("completePath: %v\n", completePath)

	newFile, err := os.Create(completePath)
	if err != nil {
		log.Printf("os.Create returned err: %v", err)
	}
	fmt.Printf("newFile: %v\n", *newFile)
	bytesWritten, err := io.Copy(newFile, data)
	if err != nil {
		log.Printf("io.Copy returned err: %v", err)
	}
	fmt.Printf("copied %d bytes\n", bytesWritten)
	// dataURL := fmt.Sprintf("data:%v;base64,%v", thumbnail.mediaType, base64Data)
	thumbnailURL := fmt.Sprintf("http://localhost:%v/assets/%v", cfg.port, filePath)
	fmt.Printf("thumbnailURL: %v", thumbnailURL)

	video.ThumbnailURL = &thumbnailURL
	if err := cfg.db.UpdateVideo(video); err != nil {
		fmt.Printf("UpdateVideo returned err: %v", err)
	}

	// returnVideo := database.Video(video)
	respondWithJSON(w, http.StatusOK, video)
}
