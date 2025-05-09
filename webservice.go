package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// RunWebService starts the REST web service
func RunWebService() {
	http.HandleFunc("/subtitles", handleSubtitles)
	http.HandleFunc("/translate", handleTranslate)

	port := 8080
	fmt.Printf("Web service running on port %d\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Error starting web service: %v\n", err)
	}
}

// handleSubtitles handles the /subtitles endpoint
func handleSubtitles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing 'path' query parameter", http.StatusBadRequest)
		return
	}

	ff, err := NewFFmpeg()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error initializing FFmpeg: %v", err), http.StatusInternalServerError)
		return
	}

	subtitleTracks, err := ff.ListSubtitleTracks(path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error listing tracks: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subtitleTracks)
}

// handleTranslate handles the /translate endpoint
func handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Path       string `json:"path"`
		TrackIndex int    `json:"track_index"`
	}
	body, _ := io.ReadAll(r.Body)
	err := json.Unmarshal(body, &request)
	if err != nil {
		fmt.Printf("Invalid JSON payload: %s\n", string(body))
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if request.Path == "" {
		http.Error(w, "Missing 'path' in request body", http.StatusBadRequest)
		return
	}

	fileType, err := DetectFileType(request.Path)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error detecting file type: %v", err), http.StatusInternalServerError)
		return
	}

	var extractedPath string
	if fileType.IsVideo() {
		ff, err := NewFFmpeg()
		if err != nil {
			http.Error(w, fmt.Sprintf("Error initializing FFmpeg: %v", err), http.StatusInternalServerError)
			return
		}

		tracks, err := ff.ListSubtitleTracks(request.Path)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error listing subtitle tracks: %v", err), http.StatusInternalServerError)
			return
		}

		if request.TrackIndex < 0 || request.TrackIndex >= len(tracks) {
			http.Error(w, "Invalid track index", http.StatusBadRequest)
			return
		}

		// Extract the subtitle track
		outputFormat := "srt"
		extractedPath, err = ff.ExtractSubtitleTrack(request.Path, request.TrackIndex, outputFormat, "en")
		if err != nil {
			http.Error(w, fmt.Sprintf("Error extracting subtitle track: %v", err), http.StatusInternalServerError)
			return
		}
	} else if fileType.IsSubtitle() {
		extractedPath = request.Path
	} else {
		http.Error(w, "Unsupported file type. Please provide an MKV video or subtitle file.", http.StatusBadRequest)
		return
	}

	// Translate the extracted subtitle
	outputPath := deriveOutputPath(extractedPath)
	translator := NewTranslator()
	err = translator.TranslateSubtitleFile(extractedPath, outputPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error translating subtitles: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":    "Translation completed successfully",
		"outputPath": outputPath,
	})
}
