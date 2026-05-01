package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 1 << 30
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Missing auth header", err)
		return
	}
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to validate token", err)
		return
	}

	videoID, err := uuid.Parse(r.PathValue("videoID"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Missing Video ID", err)
		return
	}

	err = r.ParseMultipartForm(maxUploadSize)
	http.MaxBytesReader(w, r.Body, maxUploadSize)

	videoRecord, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Video not found", err)
		return
	}
	if videoRecord.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "User is not owner", err)
		return
	}
	file, headers, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed reading form file", err)
		return
	}

	defer file.Close()
	mediaType, _, err := mime.ParseMediaType(headers.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse media type", err)
		return
	}
	if mediaType != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "unsupported media type", err)
		return
	}
	tempFile, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed creating temp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer tempFile.Close()
	_, err = io.Copy(tempFile, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to write to temp file", err)
		return
	}
	_, err = tempFile.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed moving pointer to start", err)
		return
	}

	optimizedFilePath, err := processVideoForFastStart(tempFile.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to optimize video", err)
		return
	}

	optimizedFile, err := os.Open(optimizedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to open optimized video", err)
		return
	}
	defer optimizedFile.Close()
	defer os.Remove(optimizedFilePath)

	key := make([]byte, 32)
	rand.Read(key)
	prefix, err := getVideoAspectRatio(optimizedFilePath)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get aspect ratio", err)
		return
	}
	fileName := prefix + "/" + base64.URLEncoding.EncodeToString(key) + ".mp4"
	_, err = cfg.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &fileName,
		Body:        optimizedFile,
		ContentType: &mediaType,
	})
	videoURL := fmt.Sprintf("https://%s/%s", cfg.s3CfDistribution, fileName)
	videoRecord.VideoURL = &videoURL
	err = cfg.db.UpdateVideo(videoRecord)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update video url", err)
		return
	}

}

// func generatePresingedUrl(s3Client *s3.Client, bucket, key string, expireTime time.Duration) (string, error) {
// 	presignClient := s3.NewPresignClient(s3Client)
// 	obj, err := presignClient.PresignGetObject(context.TODO(), &s3.GetObjectInput{Bucket: &bucket, Key: &key}, s3.WithPresignExpires(expireTime))
// 	if err != nil {
// 		return "", fmt.Errorf("Failed to get presigned object")
// 	}
// 	return obj.URL, nil
// }

// func (cfg *apiConfig) dbVideoTosignedVideo(video database.Video) (database.Video, error) {
// 	videoUrl := video.VideoURL
// 	if videoUrl == nil {
// 		return video, nil
// 	}
// parts := strings.Split(string(*videoUrl), ",")
// bucket := parts[0]
// key := parts[1]
// presignedUrl, err := generatePresingedUrl(cfg.s4Client, bucket, key, 1*time.Hour)
// if err != nil {
// 	return database.Video{}, fmt.Errorf("Failed to retrieve presigned url")
// }
// video.VideoURL = &presignedUrl
// 	return video, nil
// }
