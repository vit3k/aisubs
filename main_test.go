package main

import (
	"testing"
)

func TestDeriveOutputPath(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic file with no language segment",
			input:    "input.srt",
			expected: "input.pl.srt",
		},
		{
			name:     "file with eng language segment using dot separator",
			input:    "movie.eng.srt",
			expected: "movie.pl.srt",
		},
		{
			name:     "file with eng.hi language segment using dot separator",
			input:    "movie.eng.hi.srt",
			expected: "movie.pl.srt",
		},
		{
			name:     "file with en language segment using dot separator",
			input:    "movie.en.srt",
			expected: "movie.pl.srt",
		},
		{
			name:     "file with en.hi language segment using dot separator",
			input:    "movie.en.hi.srt",
			expected: "movie.pl.srt",
		},
		{
			name:     "file with eng language segment using underscore separator",
			input:    "movie_eng.srt",
			expected: "movie_pl.srt",
		},
		{
			name:     "file with eng.hi language segment using underscore separator",
			input:    "movie_eng.hi.srt",
			expected: "movie_pl.srt",
		},
		{
			name:     "file with en language segment using underscore separator",
			input:    "movie_en.srt",
			expected: "movie_pl.srt",
		},
		{
			name:     "file with en.hi language segment using underscore separator",
			input:    "movie_en.hi.srt",
			expected: "movie_pl.srt",
		},
		{
			name:     "complex filename with no language segment",
			input:    "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR.srt",
			expected: "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR.pl.srt",
		},
		{
			name:     "complex filename with eng language segment",
			input:    "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR_eng.srt",
			expected: "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR_pl.srt",
		},
		{
			name:     "complex filename with eng.hi language segment",
			input:    "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR.eng.hi.srt",
			expected: "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR.pl.srt",
		},
		{
			name:     "complex filename with en language segment",
			input:    "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR_en.srt",
			expected: "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR_pl.srt",
		},
		{
			name:     "complex filename with en.hi language segment",
			input:    "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR.en.hi.srt",
			expected: "Death of a Unicorn (2025) - [iT][WEBDL-2160p][DV HDR10Plus][EAC3 Atmos 5.1][h265]-BYNDR.pl.srt",
		},
		{
			name:     "filename with multiple segments and no language",
			input:    "movie.part1.srt",
			expected: "movie.part1.pl.srt",
		},
		{
			name:     "file with no extension",
			input:    "subtitle",
			expected: "subtitle.pl",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := deriveOutputPath(tc.input)
			if actual != tc.expected {
				t.Errorf("deriveOutputPath(%q) = %q, want %q", tc.input, actual, tc.expected)
			}
		})
	}
}