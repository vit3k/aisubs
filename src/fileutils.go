package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// IsMedia returns true if the file type is a media format (video or subtitle)
func (ft FileType) IsMedia() bool {
	return ft.IsVideo() || ft.IsSubtitle()
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

// MediaFile represents a media file found in a directory scan
type MediaFile struct {
	Path     string    `json:"path"`
	Type     string    `json:"type"`
	FileType FileType  `json:"-"` // Internal use only, not exported in JSON
	BaseName string    `json:"-"` // Base name without extension, used for grouping
	ModTime  time.Time `json:"-"` // Last modified time, used for comparison
}

// SubtitleInfo represents information about a subtitle track
type SubtitleInfo struct {
	Path         string `json:"path,omitempty"`
	TrackIndex   int    `json:"track_index"`
	Language     string `json:"language"`
	Format       string `json:"format"`
	Embedded     bool   `json:"embedded"`
	SubtitleType string `json:"type,omitempty"`
	Title        string `json:"title,omitempty"`
}

// GroupedMediaFile represents a video file with its related subtitle files
type GroupedMediaFile struct {
	ScanTime  time.Time      `json:"scan_time,omitempty"`
	VideoFile string         `json:"video_file,omitempty"`
	Subtitles []SubtitleInfo `json:"subtitles,omitempty"`
}

// FindMediaFiles recursively scans a directory for media files (videos and subtitles)
// and returns a list of grouped media files (videos with their associated subtitles)
func FindMediaFiles(dirPath string, currentCached []GroupedMediaFile) ([]GroupedMediaFile, error) {
	if currentCached == nil {
		currentCached = []GroupedMediaFile{}
	}
	// Map to store files by directory
	dirMap := make(map[string][]MediaFile)

	// Initialize FFmpeg
	ff, err := NewFFmpeg()
	if err != nil {
		return nil, fmt.Errorf("error initializing FFmpeg: %v", err)
	}

	// Collect all media files and organize them by directory
	err = filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories themselves
		if info.IsDir() {
			return nil
		}

		// Detect file type
		fileType, err := DetectFileType(path)
		if err != nil {
			// Skip files we can't analyze
			return nil
		}

		// If it's a media file, add it to our directory map
		if fileType.IsMedia() {
			// Get the directory path
			dirPath := filepath.Dir(path)

			mediaFile := MediaFile{
				Path:     path,
				Type:     fileType.String(),
				FileType: fileType,
				ModTime:  info.ModTime(),
			}

			// Add to the directory map
			dirMap[dirPath] = append(dirMap[dirPath], mediaFile)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Group media files by directory, including embedded subtitles
	return groupMediaFilesByDirectory(dirMap, ff, currentCached), nil
}

func findMediaPath(files []GroupedMediaFile, path string) (GroupedMediaFile, bool) {
	for _, file := range files {
		if file.VideoFile == path {
			return file, true
		}
	}
	return GroupedMediaFile{}, false
}

// groupMediaFilesByDirectory groups subtitle files with video files based on directory
// and also detects embedded subtitles in video files using FFmpeg
func groupMediaFilesByDirectory(dirMap map[string][]MediaFile, ff *FFmpeg, currentCached []GroupedMediaFile) []GroupedMediaFile {
	if currentCached == nil {
		currentCached = []GroupedMediaFile{}
	}
	var result []GroupedMediaFile

	// Helper to extract episode base (without extension and language/type tags)
	extractEpisodeBase := func(path string) string {
		fileName := strings.ToLower(filepath.Base(path))
		ext := strings.ToLower(filepath.Ext(fileName))
		base := strings.TrimSuffix(fileName, ext)
		parts := splitOnDelimiters(base)
		if len(parts) == 0 {
			return base
		}
		// Remove trailing language/type tags
		// Find the longest prefix that is not a language or non-language tag
		end := len(parts)
		for i := len(parts) - 1; i >= 0; i-- {
			if normalizeLanguageCode(parts[i]) != "" || nonLanguageTags[parts[i]] != "" {
				end = i
			} else {
				break
			}
		}
		return strings.Join(parts[:end], ".")
	}

	// Process each directory's files
	for _, files := range dirMap {
		// Find video files and subtitle files in this directory
		var videoFiles []MediaFile
		var subtitleFiles []MediaFile

		for _, file := range files {
			if file.FileType.IsVideo() {
				videoFiles = append(videoFiles, file)
			} else if file.FileType.IsSubtitle() {
				subtitleFiles = append(subtitleFiles, file)
			}
		}

		// Build a map from episode base to subtitle files
		subMap := make(map[string][]MediaFile)
		for _, subtitleFile := range subtitleFiles {
			epBase := extractEpisodeBase(subtitleFile.Path)
			subMap[epBase] = append(subMap[epBase], subtitleFile)
		}

		// If we have video files in this directory
		if len(videoFiles) > 0 {
			// For each video file, create a group with only matching subtitle files
			for _, videoFile := range videoFiles {
				var subtitleInfos []SubtitleInfo
				epBase := extractEpisodeBase(videoFile.Path)
				matchingSubs := subMap[epBase]

				// Add external subtitle files that match
				for _, subtitleFile := range matchingSubs {
					language, subType := determineLanguageAndTypeFromFilename(subtitleFile.Path)
					langCode := normalizeLanguageCode(language)
					subtitleInfos = append(subtitleInfos, SubtitleInfo{
						Path:         subtitleFile.Path,
						Language:     langCode,
						Format:       getSubtitleFormat(subtitleFile.FileType),
						Embedded:     false,
						SubtitleType: subType,
						Title:        languageFullName(langCode),
					})
				}
				// check if ffmpeg needs to be used
				currentCachedVideoFile, found := findMediaPath(currentCached, videoFile.Path)

				var probe = false
				if found {
					// Check if the video file has changed since last scan
					slog.Debug(videoFile.Path, "last scan time", currentCachedVideoFile.ScanTime, "current mod time", videoFile.ModTime)
					if currentCachedVideoFile.ScanTime.Before(videoFile.ModTime) {
						probe = true
					}
				} else {
					// If not found in current cache, we need to probe
					probe = true
				}
				if probe {
					// Check for embedded subtitles in the video file
					embeddedTracks, err := ff.ListSubtitleTracks(videoFile.Path)
					if err == nil {
						for _, track := range embeddedTracks {
							subType := ""
							if t, ok := nonLanguageTags[strings.ToLower(track.Language)]; ok {
								subType = t
							} else if t, ok := nonLanguageTags[strings.ToLower(track.Format)]; ok {
								subType = t
							}
							langCode := normalizeLanguageCode(track.Language)
							title := track.Title
							if title == "" {
								title = languageFullName(langCode)
							}
							subtitleInfos = append(subtitleInfos, SubtitleInfo{
								TrackIndex:   track.Index,
								Language:     langCode,
								Format:       track.Format,
								Embedded:     true,
								SubtitleType: subType,
								Title:        title,
							})
						}
					}
				}

				result = append(result, GroupedMediaFile{
					ScanTime:  time.Now(),
					VideoFile: videoFile.Path,
					Subtitles: subtitleInfos,
				})
			}
		} else if len(subtitleFiles) > 0 {
			// If we have only subtitle files in this directory
			var subtitleInfos []SubtitleInfo
			for _, subtitleFile := range subtitleFiles {
				language, subType := determineLanguageAndTypeFromFilename(subtitleFile.Path)
				langCode := normalizeLanguageCode(language)
				subtitleInfos = append(subtitleInfos, SubtitleInfo{
					Path:         subtitleFile.Path,
					Language:     langCode,
					Format:       getSubtitleFormat(subtitleFile.FileType),
					Embedded:     false,
					SubtitleType: subType,
					Title:        languageFullName(langCode),
				})
			}

			result = append(result, GroupedMediaFile{
				Subtitles: subtitleInfos,
			})
		}
	}

	return result
}

// languageFullNameMap maps ISO 639-1 codes to full language names
var languageFullNameMap = map[string]string{
	"en": "English",
	"pl": "Polish",
	"fr": "French",
	"es": "Spanish",
	"de": "German",
	"it": "Italian",
	"ja": "Japanese",
	"ko": "Korean",
	"zh": "Chinese",
	"ru": "Russian",
	"pt": "Portuguese",
	"tr": "Turkish",
	"nl": "Dutch",
	"sv": "Swedish",
	"fi": "Finnish",
	"no": "Norwegian",
	"da": "Danish",
	"hu": "Hungarian",
	"el": "Greek",
	"cs": "Czech",
	"sk": "Slovak",
	"hr": "Croatian",
	"sr": "Serbian",
	"bs": "Bosnian",
	"sl": "Slovenian",
	"bg": "Bulgarian",
	"ro": "Romanian",
	"uk": "Ukrainian",
	"he": "Hebrew",
	"ar": "Arabic",
	"hi": "Hindi",
	"bn": "Bengali",
	"ur": "Urdu",
	"fa": "Persian",
	"th": "Thai",
	"vi": "Vietnamese",
	"ms": "Malay",
	"id": "Indonesian",
	"tl": "Filipino",
	"sw": "Swahili",
	"af": "Afrikaans",
	// ... add more as needed
}

// languageCodeMap maps various language representations to canonical ISO 639-1 codes
var languageCodeMap = map[string]string{
	// English and variants
	"en": "en", "eng": "en", "english": "en",
	// Polish
	"pl": "pl", "pol": "pl", "polish": "pl", "polski": "pl",
	// French
	"fr": "fr", "fra": "fr", "fre": "fr", "french": "fr", "français": "fr",
	// Spanish
	"es": "es", "spa": "es", "spanish": "es", "español": "es",
	// German
	"de": "de", "deu": "de", "ger": "de", "german": "de", "deutsch": "de",
	// Italian
	"it": "it", "ita": "it", "italian": "it", "italiano": "it",
	// Japanese
	"ja": "ja", "jpn": "ja", "japanese": "ja", "日本語": "ja",
	// Korean
	"ko": "ko", "kor": "ko", "korean": "ko", "한국어": "ko",
	// Chinese
	"zh": "zh", "chi": "zh", "zho": "zh", "chinese": "zh", "中文": "zh", "普通话": "zh", "mandarin": "zh",
	// Russian
	"ru": "ru", "rus": "ru", "russian": "ru", "русский": "ru",
	// Portuguese
	"pt": "pt", "por": "pt", "portuguese": "pt", "português": "pt", "brazilian": "pt", "brazil": "pt", "português brasileiro": "pt",
	// Turkish
	"tr": "tr", "tur": "tr", "turkish": "tr", "türkçe": "tr",
	// Dutch
	"nl": "nl", "dut": "nl", "nld": "nl", "dutch": "nl", "nederlands": "nl",
	// Swedish
	"sv": "sv", "swe": "sv", "swedish": "sv", "svenska": "sv",
	// Finnish
	"fi": "fi", "fin": "fi", "finnish": "fi", "suomi": "fi",
	// Norwegian
	"no": "no", "nor": "no", "norwegian": "no", "norsk": "no",
	// Danish
	"da": "da", "dan": "da", "danish": "da", "dansk": "da",
	// Hungarian
	"hu": "hu", "hun": "hu", "hungarian": "hu", "magyar": "hu",
	// Greek
	"el": "el", "gre": "el", "ell": "el", "greek": "el", "ελληνικά": "el",
	// Czech
	"cs": "cs", "cze": "cs", "ces": "cs", "czech": "cs", "čeština": "cs",
	// Slovak
	"sk": "sk", "slo": "sk", "slk": "sk", "slovak": "sk", "slovenčina": "sk",
	// Croatian
	"hr": "hr", "hrv": "hr", "croatian": "hr", "hrvatski": "hr",
	// Serbian
	"sr": "sr", "srp": "sr", "serbian": "sr", "српски": "sr",
	// Bosnian
	"bs": "bs", "bos": "bs", "bosnian": "bs", "bosanski": "bs",
	// Slovenian
	"sl": "sl", "slv": "sl", "slovenian": "sl", "slovenščina": "sl",
	// Bulgarian
	"bg": "bg", "bul": "bg", "bulgarian": "bg", "български": "bg",
	// Romanian
	"ro": "ro", "rum": "ro", "ron": "ro", "romanian": "ro", "română": "ro",
	// Ukrainian
	"uk": "uk", "ukr": "uk", "ukrainian": "uk", "українська": "uk",
	// Hebrew
	"he": "he", "heb": "he", "hebrew": "he", "עברית": "he",
	// Arabic
	"ar": "ar", "ara": "ar", "arabic": "ar", "العربية": "ar",
	// Hindi
	"hi": "hi", "hin": "hi", "hindi": "hi", "हिन्दी": "hi",
	// Bengali
	"bn": "bn", "ben": "bn", "bengali": "bn", "বাংলা": "bn",
	// Urdu
	"ur": "ur", "urd": "ur", "urdu": "ur", "اُردُو": "ur",
	// Persian/Farsi
	"fa": "fa", "per": "fa", "fas": "fa", "farsi": "fa", "persian": "fa", "فارسی": "fa",
	// Thai
	"th": "th", "tha": "th", "thai": "th", "ไทย": "th",
	// Vietnamese
	"vi": "vi", "vie": "vi", "vietnamese": "vi", "tiếng việt": "vi",
	// Malay
	"ms": "ms", "may": "ms", "msa": "ms", "malay": "ms", "bahasa melayu": "ms",
	// Indonesian
	"id": "id", "ind": "id", "indonesian": "id", "bahasa indonesia": "id",
	// Filipino/Tagalog
	"tl": "tl", "tgl": "tl", "filipino": "tl", "tagalog": "tl",
	// Swahili
	"sw": "sw", "swa": "sw", "swahili": "sw", "kiswahili": "sw",
	// Afrikaans
	"af": "af", "afr": "af", "afrikaans": "af",
	// Estonian
	"et": "et", "est": "et", "estonian": "et", "eesti": "et",
	// Latvian
	"lv": "lv", "lav": "lv", "latvian": "lv", "latviešu": "lv",
	// Lithuanian
	"lt": "lt", "lit": "lt", "lithuanian": "lt", "lietuvių": "lt",
	// Icelandic
	"is": "is", "ice": "is", "isl": "is", "icelandic": "is", "íslenska": "is",
	// Maltese
	"mt": "mt", "mlt": "mt", "maltese": "mt", "malti": "mt",
	// Albanian
	"sq": "sq", "alb": "sq", "sqi": "sq", "albanian": "sq", "shqip": "sq",
	// Macedonian
	"mk": "mk", "mac": "mk", "mkd": "mk", "macedonian": "mk", "македонски": "mk",
	// Georgian
	"ka": "ka", "geo": "ka", "kat": "ka", "georgian": "ka", "ქართული": "ka",
	// Armenian
	"hy": "hy", "arm": "hy", "hye": "hy", "armenian": "hy", "հայերեն": "hy",
	// Azerbaijani
	"az": "az", "aze": "az", "azerbaijani": "az", "azərbaycan": "az",
	// Kazakh
	"kk": "kk", "kaz": "kk", "kazakh": "kk", "қазақ": "kk",
	// Uzbek
	"uz": "uz", "uzb": "uz", "uzbek": "uz", "oʻzbek": "uz",
	// Turkmen
	"tk": "tk", "tuk": "tk", "turkmen": "tk", "türkmen": "tk",
	// Pashto
	"ps": "ps", "pus": "ps", "pashto": "ps", "پښتو": "ps",
	// Kurdish
	"ku": "ku", "kur": "ku", "kurdish": "ku", "kurdî": "ku",
	// Somali
	"so": "so", "som": "so", "somali": "so", "af-soomaali": "so",
	// Nepali
	"ne": "ne", "nep": "ne", "nepali": "ne", "नेपाली": "ne",
	// Sinhala
	"si": "si", "sin": "si", "sinhala": "si", "සිංහල": "si",
	// Lao
	"lo": "lo", "lao": "lo", "ລາວ": "lo",
	// Khmer
	"km": "km", "khm": "km", "khmer": "km", "ភាសាខ្មែរ": "km",
	// Burmese
	"my": "my", "bur": "my", "mya": "my", "burmese": "my", "မြန်မာ": "my",
	// Mongolian
	"mn": "mn", "mon": "mn", "mongolian": "mn", "монгол": "mn",
	// Tibetan
	"bo": "bo", "tib": "bo", "bod": "bo", "tibetan": "bo", "བོད་སྐད་": "bo",
	// Yiddish
	"yi": "yi", "yid": "yi", "yiddish": "yi", "ייִדיש": "yi",
	// Haitian Creole
	"ht": "ht", "hat": "ht", "haitian": "ht", "haitian creole": "ht", "kreyòl ayisyen": "ht",
	// Luxembourgish
	"lb": "lb", "ltz": "lb", "luxembourgish": "lb", "lëtzebuergesch": "lb",
	// Catalan
	"ca": "ca", "cat": "ca", "catalan": "ca", "català": "ca",
	// Galician
	"gl": "gl", "glg": "gl", "galician": "gl", "galego": "gl",
	// Basque
	"eu": "eu", "baq": "eu", "eus": "eu", "basque": "eu", "euskara": "eu",
	// Welsh
	"cy": "cy", "wel": "cy", "cym": "cy", "welsh": "cy", "cymraeg": "cy",
	// Irish
	"ga": "ga", "gle": "ga", "irish": "ga", "gaeilge": "ga",
	// Scottish Gaelic
	"gd": "gd", "gla": "gd", "scottish gaelic": "gd", "gàidhlig": "gd",
	// Breton
	"br": "br", "bre": "br", "breton": "br", "brezhoneg": "br",
	// Corsican
	"co": "co", "cos": "co", "corsican": "co", "corsu": "co",
	// Occitan
	"oc": "oc", "oci": "oc", "occitan": "oc", "occitan (post 1500)": "oc",
	// Frisian
	"fy": "fy", "fry": "fy", "frisian": "fy", "frysk": "fy",
	// Manx
	"gv": "gv", "glv": "gv", "manx": "gv", "gaelg": "gv",
	// Esperanto
	"eo": "eo", "epo": "eo", "esperanto": "eo",
	// Interlingua
	"ia": "ia", "ina": "ia", "interlingua": "ia",
	// Latin
	"la": "la", "lat": "la", "latin": "la",
	// Sanskrit
	"sa": "sa", "san": "sa", "sanskrit": "sa", "संस्कृतम्": "sa",
	// Hawaiian
	"haw": "haw", "hawaiian": "haw", "ʻŌlelo Hawaiʻi": "haw",
	// Samoan
	"sm": "sm", "smo": "sm", "samoan": "sm", "gagana fa'a Samoa": "sm",
	// Tahitian
	"ty": "ty", "tah": "ty", "tahitian": "ty", "reo tahiti": "ty",
	// Maori
	"mi": "mi", "mao": "mi", "mri": "mi", "maori": "mi", "te reo māori": "mi",
	// Tongan
	"to": "to", "ton": "to", "tongan": "to", "lea fakatonga": "to",
	// Fijian
	"fj": "fj", "fij": "fj", "fijian": "fj", "vosa vaka-Viti": "fj",
	// Greenlandic
	"kl": "kl", "kal": "kl", "greenlandic": "kl", "kalaallisut": "kl",
	// Inuktitut
	"iu": "iu", "iku": "iu", "inuktitut": "iu", "ᐃᓄᒃᑎᑐᑦ": "iu",
	// Cherokee
	"chr": "chr", "cherokee": "chr", "ᏣᎳᎩ": "chr",
	// Zulu
	"zu": "zu", "zul": "zu", "zulu": "zu", "isiZulu": "zu",
	// Xhosa
	"xh": "xh", "xho": "xh", "xhosa": "xh", "isiXhosa": "xh",
	// Sesotho
	"st": "st", "sot": "st", "sesotho": "st",
	// Tswana
	"tn": "tn", "tsn": "tn", "tswana": "tn",
	// Venda
	"ve": "ve", "ven": "ve", "venda": "ve",
	// Tsonga
	"ts": "ts", "tso": "ts", "tsonga": "ts",
	// Swati
	"ss": "ss", "ssw": "ss", "swati": "ss",
	// Ndebele
	"nr": "nr", "nbl": "nr", "ndebele": "nr",
	// Shona
	"sn": "sn", "sna": "sn", "shona": "sn",
	// Wolof
	"wo": "wo", "wol": "wo", "wolof": "wo",
	// Igbo
	"ig": "ig", "ibo": "ig", "igbo": "ig",
	// Yoruba
	"yo": "yo", "yor": "yo", "yoruba": "yo",
	// Hausa
	"ha": "ha", "hau": "ha", "hausa": "ha",
	// Amharic
	"am": "am", "amh": "am", "amharic": "am", "አማርኛ": "am",
	// Tigrinya
	"ti": "ti", "tir": "ti", "tigrinya": "ti", "ትግርኛ": "ti",
	// Oromo
	"om": "om", "orm": "om", "oromo": "om",
	// Malagasy
	"mg": "mg", "mlg": "mg", "malagasy": "mg",
	// Quechua
	"qu": "qu", "que": "qu", "quechua": "qu",
	// Aymara
	"ay": "ay", "aym": "ay", "aymara": "ay",
	// Nahuatl
	"nah": "nah", "nahuatl": "nah",
	// Mapudungun
	"arn": "arn", "mapudungun": "arn",
	// Others can be added as needed
}

// nonLanguageTags is a set of known non-language subtitle tags
var nonLanguageTags = map[string]string{
	"hi":     "hearing impaired",
	"cc":     "closed captions",
	"sdh":    "subtitles for the deaf and hard of hearing",
	"forced": "forced",
	"sign":   "sign language",
	"deaf":   "deaf",
	"hoh":    "hard of hearing",
}

// normalizeLanguageCode returns the canonical ISO 639-1 code for a given code or name
func normalizeLanguageCode(code string) string {
	code = strings.ToLower(strings.TrimSpace(code))
	if val, ok := languageCodeMap[code]; ok {
		return val
	}
	return ""
}

// languageFullName returns the full language name for a given ISO 639-1 code
func languageFullName(code string) string {
	if name, ok := languageFullNameMap[code]; ok {
		return name
	}
	return ""
}

// determineLanguageAndTypeFromFilename extracts and normalizes language code and subtitle type from subtitle filename
func determineLanguageAndTypeFromFilename(filePath string) (string, string) {
	fileName := strings.ToLower(filepath.Base(filePath))
	ext := strings.ToLower(filepath.Ext(fileName))
	base := strings.TrimSuffix(fileName, ext)

	parts := strings.Split(base, ".")
	lang := ""
	subType := ""
	for _, part := range parts {
		if lang == "" {
			code := normalizeLanguageCode(part)
			if code != "" {
				lang = code
				continue
			}
		}
		if subType == "" {
			if t, isNonLang := nonLanguageTags[part]; isNonLang {
				subType = t
			}
		}
		if lang != "" && subType != "" {
			break
		}
	}

	// Fallback: try other delimiters if language not found
	if lang == "" {
		parts = strings.FieldsFunc(base, func(r rune) bool {
			return r == '_' || r == '-' || r == ' '
		})
		for _, part := range parts {
			if lang == "" {
				code := normalizeLanguageCode(part)
				if code != "" {
					lang = code
					continue
				}
			}
			if subType == "" {
				if t, isNonLang := nonLanguageTags[part]; isNonLang {
					subType = t
				}
			}
			if lang != "" && subType != "" {
				break
			}
		}
	}
	return lang, subType
}

// splitOnDelimiters splits a string on ., _, -, and space
func splitOnDelimiters(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '.' || r == '_' || r == '-' || r == ' '
	})
}

// getSubtitleFormat converts the FileType to a subtitle format string
func getSubtitleFormat(fileType FileType) string {
	switch fileType {
	case FileTypeSubtitleSRT:
		return "subrip"
	case FileTypeSubtitleSSA:
		return "ssa"
	case FileTypeSubtitleASS:
		return "ass"
	default:
		return "unknown"
	}
}
