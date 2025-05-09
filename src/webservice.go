package main

import (
	"encoding/json"
	"fmt"
	"io"
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
	http.HandleFunc("/subtitles", handleSubtitles)
	http.HandleFunc("/translate", handleTranslate)
	http.HandleFunc("/job", handleJob)

	port := 8080
	fmt.Printf("Web service running on port %d\n", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil {
		fmt.Printf("Error starting web service: %v\n", err)
	}
}

// handleJob handles the /job endpoint for checking job status
func handleJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Invalid request method", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

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
	if r.Method != http.MethodGet {
		sendErrorResponse(w, "Invalid request method", "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

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
		fmt.Printf("Error initializing FFmpeg: %v\n", err)
		return
	}

	fmt.Printf("Scanning file for subtitles: %s\n", path)
	subtitleTracks, err := ff.ListSubtitleTracks(path)
	if err != nil {
		errorMsg := fmt.Sprintf("Error listing subtitle tracks: %v", err)
		sendErrorResponse(w, "Subtitle track error", errorMsg, http.StatusInternalServerError)
		fmt.Println(errorMsg)
		return
	}

	if len(subtitleTracks) == 0 {
		sendErrorResponse(w, "No subtitles found", fmt.Sprintf("No subtitle tracks found in the media file: %s", path), http.StatusNotFound)
		fmt.Printf("No subtitle tracks found in: %s\n", path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subtitleTracks)
}

// handleTranslate handles the /translate endpoint
func handleTranslate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendErrorResponse(w, "Invalid request method", "Only POST method is allowed", http.StatusMethodNotAllowed)
		return
	}

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
		fmt.Printf("Invalid JSON payload: %s\n", string(body))
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
		fmt.Println(errorMsg)
		return
	} else if err != nil {
		errorMsg := fmt.Sprintf("Error accessing file: %v", err)
		sendErrorResponse(w, "File access error", errorMsg, http.StatusInternalServerError)
		fmt.Println(errorMsg)
		return
	}
	
	// Verify the track index is non-negative
	if request.TrackIndex < 0 {
		errorMsg := fmt.Sprintf("Invalid track index: %d", request.TrackIndex)
		sendErrorResponse(w, "Invalid parameter", errorMsg, http.StatusBadRequest)
		fmt.Println(errorMsg)
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
