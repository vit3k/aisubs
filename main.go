package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
		tracks, err := ff.ListSubtitleTracks(inputPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error listing subtitle tracks: %v\n", err)
			os.Exit(1)
		}

		if len(tracks) == 0 {
			fmt.Println("No subtitle tracks found in the video file.")
			os.Exit(1)
		}

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
		extractedPath, err := ff.ExtractSubtitleTrack(inputPath, trackIndex, outputFormat, langCode)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error extracting subtitle track: %v\n", err)
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
		os.Exit(1)
	}

	fmt.Printf("\nTranslation completed successfully!\n")
	fmt.Printf("Polish subtitles saved to: %s\n", outputPath)
	
	// Clean up the extracted subtitle file if it's not the original input
	if fileType.IsVideo() && subtitlePath != inputPath {
		fmt.Printf("Note: Temporary subtitle file %s was used for translation.\n", subtitlePath)
	}
}

