package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
)

// ErrorResponse defines the structure of error responses
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
	Code    int    `json:"code"`
}

// sendErrorResponse sends a consistent JSON error response
func sendErrorResponse(w http.ResponseWriter, message string, details string, statusCode int) {
	resp := ErrorResponse{
		Error:   message,
		Details: details,
		Code:    statusCode,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// RunWebService starts the REST web service
func RunWebService() {
	// Log the configured media paths
	slog.Info("Configured media paths")
	for name, pathConfig := range GetAllMediaPaths() {
		slog.Info("Media path", "name", name, "path", pathConfig.Path, "description", pathConfig.Description)
	}
	mux := http.NewServeMux()

	mux.HandleFunc("GET /subtitles/", handleSubtitles)
	mux.HandleFunc("POST /translate/", handleTranslate)
	mux.HandleFunc("GET /job/", handleJob)
	mux.HandleFunc("GET /media/", handleMedia)

	port := GetPort()
	slog.Info("Web service running", "port", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), mux)
	if err != nil {
		slog.Error("Failed to start web service", "error", err)
	}
}

// handleJob handles the /job endpoint for checking job status
func handleJob(w http.ResponseWriter, r *http.Request) {
	// Get the job ID from the query parameters
	jobID := r.URL.Query().Get("id")
	if jobID == "" {
		sendErrorResponse(w, "Missing parameter", "The 'id' query parameter is required", http.StatusBadRequest)
		return
	}

	// Get the job from the job manager
	jm := GetJobManager()
	job, err := jm.GetJob(jobID)
	if err != nil {
		sendErrorResponse(w, "Job not found", err.Error(), http.StatusNotFound)
		return
	}

	// Return the job status to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(job)
}

// handleSubtitles handles the /subtitles endpoint
func handleSubtitles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		sendErrorResponse(w, "Missing parameter", "The 'path' query parameter is required", http.StatusBadRequest)
		return
	}

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		sendErrorResponse(w, "File not found", fmt.Sprintf("The file '%s' does not exist", path), http.StatusNotFound)
		return
	} else if err != nil {
		sendErrorResponse(w, "File access error", err.Error(), http.StatusInternalServerError)
		return
	}

	ff, err := NewFFmpeg()
	if err != nil {
		sendErrorResponse(w, "FFmpeg initialization error", err.Error(), http.StatusInternalServerError)
		slog.Error("Failed to initialize FFmpeg", "error", err)
		return
	}

	slog.Info("Scanning file for subtitles", "path", path)
	subtitleTracks, err := ff.ListSubtitleTracks(path)
	if err != nil {
		errorMsg := fmt.Sprintf("Error listing subtitle tracks: %v", err)
		sendErrorResponse(w, "Subtitle track error", errorMsg, http.StatusInternalServerError)
		slog.Error("Failed to list subtitle tracks", "error", err)
		return
	}

	if len(subtitleTracks) == 0 {
		sendErrorResponse(w, "No subtitles found", fmt.Sprintf("No subtitle tracks found in the media file: %s", path), http.StatusNotFound)
		slog.Info("No subtitle tracks found", "path", path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subtitleTracks)
}

// handleTranslate handles the /translate endpoint
func handleTranslate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path       string `json:"path"`
		TrackIndex int    `json:"track_index"`
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		sendErrorResponse(w, "Request body error", err.Error(), http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &request)
	if err != nil {
		slog.Error("Invalid JSON payload", "body", string(body), "error", err)
		sendErrorResponse(w, "Invalid JSON", "The request body could not be parsed as valid JSON", http.StatusBadRequest)
		return
	}

	if request.Path == "" {
		sendErrorResponse(w, "Missing field", "The 'path' field is required in the request body", http.StatusBadRequest)
		return
	}

	// Verify the file exists before processing
	if _, err := os.Stat(request.Path); os.IsNotExist(err) {
		errorMsg := fmt.Sprintf("File not found: %s", request.Path)
		sendErrorResponse(w, "File not found", errorMsg, http.StatusNotFound)
		slog.Error("File not found", "path", request.Path)
		return
	} else if err != nil {
		errorMsg := fmt.Sprintf("Error accessing file: %v", err)
		sendErrorResponse(w, "File access error", errorMsg, http.StatusInternalServerError)
		slog.Error("Error accessing file", "path", request.Path, "error", err)
		return
	}

	// Verify the track index is non-negative
	if request.TrackIndex < 0 {
		errorMsg := fmt.Sprintf("Invalid track index: %d", request.TrackIndex)
		sendErrorResponse(w, "Invalid parameter", errorMsg, http.StatusBadRequest)
		slog.Error("Invalid track index", "index", request.TrackIndex)
		return
	}

	// Create a new job and start processing it
	jm := GetJobManager()
	job := jm.CreateJob(request.Path, request.TrackIndex)
	jm.ProcessJob(job.ID)

	// Return the job ID to the client
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Translation job created",
		"job_id":  job.ID,
	})
}

// handleMedia handles the /media endpoint for listing media files in a directory
func handleMedia(w http.ResponseWriter, r *http.Request) {
	slog.Info("Handling media request")
	name := r.URL.Query().Get("name")

	if name == "" {
		sendErrorResponse(w, "Missing parameter", "The 'name' query parameter is required", http.StatusBadRequest)
		return
	}

	mediaPath, err := GetMediaPath(name)
	if err != nil {
		sendErrorResponse(w, "Invalid media path name", fmt.Sprintf("No media path named '%s' found in configuration", name), http.StatusBadRequest)
		return
	}

	// Check if force refresh is requested
	forceRefresh := r.URL.Query().Get("refresh") == "true"

	// Check if the directory exists
	fileInfo, err := os.Stat(mediaPath)
	if os.IsNotExist(err) {
		sendErrorResponse(w, "Directory not found", fmt.Sprintf("The directory '%s' does not exist", mediaPath), http.StatusNotFound)
		return
	} else if err != nil {
		sendErrorResponse(w, "Directory access error", err.Error(), http.StatusInternalServerError)
		return
	}

	// Verify that the path is actually a directory
	if !fileInfo.IsDir() {
		sendErrorResponse(w, "Invalid path", fmt.Sprintf("The path '%s' is not a directory", mediaPath), http.StatusBadRequest)
		return
	}

	// Get database connection
	db := GetDB()
	var groupedMediaFiles []GroupedMediaFile
	var err2 error

	if db != nil {
		slog.Info("Scanning directory for media files", "path", mediaPath, "using_cache", !forceRefresh)

		if forceRefresh {
			// Force refresh - scan and update cache
			groupedMediaFiles, err2 = RefreshMediaFilesCache(db, mediaPath)
		} else {
			// Try to use cache first, fall back to scanning if needed
			groupedMediaFiles, err2 = FindMediaFilesWithCache(db, mediaPath)
		}
	} else {
		// No database available, just scan directly
		slog.Info("Scanning directory for media files", "path", mediaPath, "message", "no cache available")
		groupedMediaFiles, err2 = FindMediaFiles(mediaPath, nil)
	}

	if err2 != nil {
		errorMsg := fmt.Sprintf("Error scanning directory: %v", err2)
		sendErrorResponse(w, "Directory scan error", errorMsg, http.StatusInternalServerError)
		slog.Error("Error scanning directory", "path", mediaPath, "error", err2)
		return
	}

	if len(groupedMediaFiles) == 0 {
		slog.Info("No media files found", "path", mediaPath)
		// Return an empty array rather than an error for this case
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]GroupedMediaFile{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(groupedMediaFiles)
}
