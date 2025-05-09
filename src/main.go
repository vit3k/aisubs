package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Global job manager
var jobManager *JobManager
var jobManagerOnce sync.Once

// GetJobManager returns the singleton job manager instance
func GetJobManager() *JobManager {
	jobManagerOnce.Do(func() {
		jobManager = NewJobManager()
	})
	return jobManager
}

func main() {
	// Check if an input file or service flag is provided
	if len(os.Args) < 2 {
		fmt.Println("Usage: aisubs2 <input_file> | -s")
		fmt.Println("  input_file: MKV file or subtitle file")
		fmt.Println("    - If MKV: extracts first English subtitle, then translates to Polish")
		fmt.Println("    - If subtitle: translates directly to Polish")
		fmt.Println("  -s: Runs the web service")
		os.Exit(1)
	}

	// Initialize the job manager
	_ = GetJobManager()

	// Check if the -s flag is provided
	if os.Args[1] == "-s" {
		fmt.Println("Starting web service...")
		RunWebService()
		return
	}

	inputPath := os.Args[1]

	// Check if file exists
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: File '%s' does not exist\n", inputPath)
		os.Exit(1)
	}

	// Initialize translator
	translator := NewTranslator()

	// Initialize FFmpeg
	ff, err := NewFFmpeg()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing FFmpeg: %v\n", err)
		fmt.Fprintf(os.Stderr, "Please ensure ffmpeg is installed and available in your PATH\n")
		os.Exit(1)
	}

	// Verify the input file exists and is readable
	fileInfo, err := os.Stat(inputPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Input file '%s' does not exist\n", inputPath)
		} else if os.IsPermission(err) {
			fmt.Fprintf(os.Stderr, "Error: Permission denied when accessing '%s'\n", inputPath)
		} else {
			fmt.Fprintf(os.Stderr, "Error accessing input file: %v\n", err)
		}
		os.Exit(1)
	}
	
	// Make sure it's a regular file, not a directory
	if fileInfo.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: '%s' is a directory, not a file\n", inputPath)
		os.Exit(1)
	}
	
	// Detect file type
	fileType, err := DetectFileType(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error detecting file type: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Detected file type: %s\n", fileType)

	var subtitlePath string

	// Process based on file type
	if fileType.IsVideo() {
		fmt.Println("Analyzing video file for subtitles...")
		
		// List all subtitle tracks
		fmt.Println("Scanning video file for subtitle tracks...")
		tracks, err := ff.ListSubtitleTracks(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing subtitle tracks: %v\n", err)
			fmt.Fprintf(os.Stderr, "Make sure the file is a valid video file and is not corrupted\n")
			os.Exit(1)
		}

		if len(tracks) == 0 {
			fmt.Fprintf(os.Stderr, "No subtitle tracks found in the video file '%s'.\n", inputPath)
			fmt.Fprintf(os.Stderr, "Please provide a video file that contains subtitles.\n")
			os.Exit(1)
		}
		
		fmt.Printf("Found %d subtitle tracks\n", len(tracks))

		// Find the first English subtitle track
		trackIndex := FindFirstEnglishSubtitleTrack(tracks)
		if trackIndex == -1 {
			fmt.Println("No English subtitle tracks found. Using first available track.")
			trackIndex = 0
		}

		// Determine output format
		outputFormat := "srt"
		if tracks[trackIndex].Format == "ass" || tracks[trackIndex].Format == "ssa" {
			outputFormat = "ass"
		}

		// Get language code from the track
		langCode := "en"
		if tracks[trackIndex].Language != "" {
			langCode = tracks[trackIndex].Language
		}
		fmt.Printf("Extracting subtitle track %d (%s, %s)...\n", 
				trackIndex, langCode, tracks[trackIndex].Format)
		
		// Extract the selected subtitle track
		fmt.Printf("Extracting subtitle track %d (%s) to %s format...\n", trackIndex, langCode, outputFormat)
		extractedPath, err := ff.ExtractSubtitleTrack(inputPath, trackIndex, outputFormat, langCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting subtitle track: %v\n", err)
			
			// Provide more detailed error information
			if strings.Contains(err.Error(), "invalid subtitle track index") {
				fmt.Fprintf(os.Stderr, "The specified track index %d may not exist in this file\n", trackIndex)
			} else if strings.Contains(err.Error(), "permission denied") {
				fmt.Fprintf(os.Stderr, "Permission denied when writing output file. Check your write permissions.\n")
			} else {
				fmt.Fprintf(os.Stderr, "FFmpeg failed to extract subtitles. The file may be corrupted or in an unsupported format.\n")
			}
			
			os.Exit(1)
		}
		
		// Verify the extracted file exists and has content
		if fileInfo, err := os.Stat(extractedPath); err != nil || fileInfo.Size() == 0 {
			fmt.Fprintf(os.Stderr, "Error: Subtitle extraction failed. The output file is empty or not created.\n")
			os.Exit(1)
		}

		fmt.Printf("Subtitle extracted to: %s\n", extractedPath)
		subtitlePath = extractedPath
	} else if fileType.IsSubtitle() {
		// Use the input file directly
		subtitlePath = inputPath
	} else {
		fmt.Fprintf(os.Stderr, "Error: Unsupported file type. Please provide an MKV video or subtitle file.\n")
		os.Exit(1)
	}

	// For video files, we want to derive the Polish output path from the original video file,
	// not from the extracted subtitle file, to maintain consistent naming
	var outputPath string
	if fileType.IsVideo() {
		// Get the output format from the temporary subtitle file extension
		outputFormat := filepath.Ext(subtitlePath)
		if outputFormat != "" {
			outputFormat = outputFormat[1:] // Remove the leading dot
		} else {
			outputFormat = "srt" // Default to srt if no extension found
		}
		
		// Derive output path from original video file
		outputPath = deriveOutputPath(inputPath)
		// Change the extension to match the subtitle format
		outputPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + "." + outputFormat
	} else {
		// For subtitle files, derive directly from the subtitle path
		outputPath = deriveOutputPath(subtitlePath)
	}
	
	// Translate the subtitle file
	fmt.Printf("Translating subtitles to Polish...\n")
	err = translator.TranslateSubtitleFile(subtitlePath, outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error translating subtitles: %v\n", err)
		
		// If the error occurred after extracting subtitles from a video,
		// let the user know the temporary file that was created
		if fileType.IsVideo() && subtitlePath != inputPath {
			fmt.Fprintf(os.Stderr, "Subtitles were extracted to '%s' but translation failed.\n", subtitlePath)
			fmt.Fprintf(os.Stderr, "You may try manually translating this file.\n")
		}
		
		os.Exit(1)
	}

	fmt.Printf("\nTranslation completed successfully!\n")
	fmt.Printf("Polish subtitles saved to: %s\n", outputPath)
	
	// Clean up the extracted subtitle file if it's not the original input
	if fileType.IsVideo() && subtitlePath != inputPath {
		fmt.Printf("Note: Temporary subtitle file %s was used for translation.\n", subtitlePath)
	}
}

