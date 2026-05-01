package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
)

type FFProbeOutput struct {
	Streams []struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"streams"`
}

func getVideoAspectRatio(filePath string) (string, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-print_format", "json", "-show_streams", filePath)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		log.Printf("ffprobe stderr: %s", stderr.String())
		return "", fmt.Errorf("Failed to run command %w", err)

	}
	var videoData FFProbeOutput
	err = json.Unmarshal([]byte(out.Bytes()), &videoData)
	if err != nil {
		return "", fmt.Errorf("Failed to unmarshal videoData %w", err)
	}
	height := videoData.Streams[0].Height
	width := videoData.Streams[0].Width
	ratio := width / height
	switch ratio {
	case 1:
		return "landscape", nil
	case 0:
		return "portrait", nil
	default:
		return "other", nil
	}

}

func processVideoForFastStart(filePath string) (string, error) {
	outputFilePath := filePath + ".processing"

	cmd := exec.Command("ffmpeg", "-i", filePath, "-c", "copy", "-movflags", "faststart", "-f", "mp4", outputFilePath)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		log.Printf("ffmpeg stderr: %s", stderr.String())
		return "", fmt.Errorf("Failed to run command ffmpeg %w", err)
	}
	return outputFilePath, nil
}
