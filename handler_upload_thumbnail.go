package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"

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
	dataSlice, err := io.ReadAll(data)
	if err != nil {
		log.Printf("io.ReadAll returned err: %v", err)
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
	thumbnail := thumbnail{
		data:      dataSlice,
		mediaType: mediaType,
	}
	base64Data := base64.StdEncoding.EncodeToString(thumbnail.data)

	// videoThumbnails[video.ID] = thumbnail

	// fmt.Printf("PORT: %v\n", cfg.port)
	// thumbnailURL := fmt.Sprintf("http://localhost:%v/api/thumbnails/%v", cfg.port, video.ID)
	dataURL := fmt.Sprintf("data:%v;base64,%v", thumbnail.mediaType, base64Data)

	video.ThumbnailURL = &dataURL
	if err := cfg.db.UpdateVideo(video); err != nil {
		fmt.Printf("UpdateVideo returned err: %v", err)
	}

	// returnVideo := database.Video(video)
	respondWithJSON(w, http.StatusOK, video)
}
