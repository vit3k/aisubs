# aisubs2 - Automatic Subtitle Extraction and Translation

aisubs2 is a simple command-line tool that extracts subtitles from video files and translates them to Polish using OpenAI's API. It features a simplified one-parameter interface that automatically detects file types and performs the appropriate actions.

## Features

- **Simplified Interface** - Just provide a single input file and the tool does the rest
- **Automatic File Type Detection** - Works with both video files and subtitle files
- **Automatic English Subtitle Extraction** - Finds and extracts the first English subtitle track from videos
- **Polish Translation** - Translates subtitles using OpenAI's API
- **Preserves Subtitle Formats** - Handles SRT, SSA, and ASS subtitle formats

## Installation

### Prerequisites

- Go 1.16 or higher
- FFmpeg installed and available in your system's PATH
- OpenAI API key

### Building from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/aisubs2.git
cd aisubs2

# Set your OpenAI API key
export OPENAI_API_KEY=your_api_key_here

# Build the tool
go build

# Run the tool
./aisubs2 your_file.mkv
```

## Usage

The tool now accepts only one parameter - the input file path:

```bash
./aisubs2 <input_file>
```

Where `input_file` can be either:
- A video file (MKV, MP4, AVI) containing subtitles
- A subtitle file (SRT, SSA, ASS)

### Examples

#### Processing a Video File

```bash
./aisubs2 movie.mkv
```

This will:
1. Detect that `movie.mkv` is a video file
2. Find and extract the first English subtitle track
3. Translate the extracted subtitles to Polish
4. Save the Polish subtitles as `movie.pl.srt`

#### Processing a Subtitle File

```bash
./aisubs2 subtitle.srt
```

This will:
1. Detect that `subtitle.srt` is a subtitle file
2. Translate the subtitles directly to Polish
3. Save the Polish subtitles as `subtitle.pl.srt`

## How It Works

1. **File Type Detection** - The tool examines the file header and extension to determine if it's a video or subtitle file
2. **Video Processing** - For video files:
   - Lists all subtitle tracks in the file
   - Finds the first English subtitle track (or uses the first available track if no English tracks are found)
   - Extracts the subtitle track to a temporary file
3. **Translation** - The tool uses OpenAI's API to translate the subtitles to Polish
4. **Output** - The translated subtitles are saved with the following naming convention:
   - If the original filename has a language code (e.g., `movie.en.srt` or `movie_eng.srt`), it is replaced with "pl" (e.g., `movie.pl.srt` or `movie_pl.srt`)
   - If no language code exists, ".pl" is added before the extension (e.g., `movie.srt` → `movie.pl.srt`)
   - For video files, the output follows the same pattern but maintains the original filename (e.g., `movie.mkv` → `movie.pl.srt`)

## Architecture

The tool is built with a modular architecture:
- `FFmpeg` - Encapsulates FFmpeg functionality for media processing
- `Translator` - Handles subtitle translation using OpenAI's API
- File utilities - Detects file types and handles file operations

## License

This project is licensed under the MIT License.

## Acknowledgments

- Uses FFmpeg for media processing and subtitle extraction
- Uses OpenAI's API for high-quality translation
- Inspired by the need for a simple tool to watch foreign films with Polish subtitles