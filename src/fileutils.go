package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// FileType represents the detected type of a file
type FileType int

const (
	FileTypeUnknown FileType = iota
	FileTypeMKV
	FileTypeMP4
	FileTypeAVI
	FileTypeSubtitleSRT
	FileTypeSubtitleSSA
	FileTypeSubtitleASS
)

// String returns the string representation of the FileType
func (ft FileType) String() string {
	switch ft {
	case FileTypeMKV:
		return "MKV Video"
	case FileTypeMP4:
		return "MP4 Video"
	case FileTypeAVI:
		return "AVI Video"
	case FileTypeSubtitleSRT:
		return "SRT Subtitle"
	case FileTypeSubtitleSSA:
		return "SSA Subtitle"
	case FileTypeSubtitleASS:
		return "ASS Subtitle"
	default:
		return "Unknown"
	}
}

// IsVideo returns true if the file type is a video format
func (ft FileType) IsVideo() bool {
	return ft == FileTypeMKV || ft == FileTypeMP4 || ft == FileTypeAVI
}

// IsSubtitle returns true if the file type is a subtitle format
func (ft FileType) IsSubtitle() bool {
	return ft == FileTypeSubtitleSRT || ft == FileTypeSubtitleSSA || ft == FileTypeSubtitleASS
}

// DetectFileType detects the type of file based on its header and/or extension
func DetectFileType(filePath string) (FileType, error) {
	// First, try to detect by file extension
	ext := strings.ToLower(filepath.Ext(filePath))
	
	switch ext {
	case ".mkv":
		return FileTypeMKV, nil
	case ".mp4":
		return FileTypeMP4, nil
	case ".avi":
		return FileTypeAVI, nil
	case ".srt":
		return FileTypeSubtitleSRT, nil
	case ".ssa":
		return FileTypeSubtitleSSA, nil
	case ".ass":
		return FileTypeSubtitleASS, nil
	}

	// If extension doesn't provide enough information, check file header
	file, err := os.Open(filePath)
	if err != nil {
		return FileTypeUnknown, err
	}
	defer file.Close()

	// Read the first 12 bytes to check file signature
	header := make([]byte, 12)
	_, err = io.ReadFull(file, header)
	if err != nil {
		return FileTypeUnknown, err
	}

	// Check for MKV signature
	if bytes.Equal(header[0:4], []byte{0x1A, 0x45, 0xDF, 0xA3}) {
		return FileTypeMKV, nil
	}
	
	// MP4 signature (ftyp...)
	if bytes.Equal(header[4:8], []byte("ftyp")) {
		return FileTypeMP4, nil
	}
	
	// Reset file pointer to start
	_, err = file.Seek(0, 0)
	if err != nil {
		return FileTypeUnknown, err
	}
	
	// Try to detect subtitle format by reading first few lines
	scanner := bufio.NewScanner(file)
	lineCount := 0
	
	for scanner.Scan() && lineCount < 10 {
		line := scanner.Text()
		lineCount++
		
		// Look for SRT format indicator (numeric index as first non-empty line)
		if lineCount == 1 && isNumeric(line) {
			return FileTypeSubtitleSRT, nil
		}
		
		// Look for SSA/ASS format indicator
		if strings.Contains(line, "[Script Info]") {
			if strings.Contains(line, "SSA") {
				return FileTypeSubtitleSSA, nil
			}
			return FileTypeSubtitleASS, nil
		}
	}
	
	// If we've reached here, we couldn't detect the file type
	return FileTypeUnknown, nil
}

// FindFirstEnglishSubtitleTrack finds the first English subtitle track in a video file
func FindFirstEnglishSubtitleTrack(tracks []SubtitleTrack) int {
	for i, track := range tracks {
		// Check for English language codes
		lang := strings.ToLower(track.Language)
		if lang == "eng" || lang == "en" || lang == "english" {
			return i
		}
	}
	
	// If no English track found, return the first track (if any)
	if len(tracks) > 0 {
		return 0
	}
	
	return -1 // No tracks found
}

// isNumeric checks if a string contains only numeric characters
func isNumeric(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}