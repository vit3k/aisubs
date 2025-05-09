package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"sync"

	"github.com/asticode/go-astisub"
	"github.com/invopop/jsonschema"
	"github.com/openai/openai-go"
)

// LineItem represents a single text item within a subtitle line
type LineItem struct {
	Text string `json:"text"`
}

// Line represents a line of text in a subtitle
type Line struct {
	Items []LineItem `json:"items"`
}

// Subtitle represents a subtitle entry with an index and lines of text
type Subtitle struct {
	Index int    `json:"index"`
	Lines []Line `json:"lines"`
}

// TranslationResponse represents the response format from the translation API
type TranslationResponse struct {
	Subtitles []Subtitle `json:"subtitles"`
}

// TranslationConfig holds configuration for translation operations
type TranslationConfig struct {
	BatchSize        int    // Number of subtitles to process in each batch
	ConcurrencyLimit int    // Maximum number of concurrent translation requests
	TargetLanguage   string // Target language for translation (default: "polish")
	Model            string // OpenAI model to use
}

// DefaultTranslationConfig returns a default configuration for translation
func DefaultTranslationConfig() TranslationConfig {
	return TranslationConfig{
		BatchSize:        30,
		ConcurrencyLimit: 5,
		TargetLanguage:   "polish",
		Model:            openai.ChatModelGPT4oMini,
	}
}

// GenerateSchema generates a JSON schema for the specified type
func GenerateSchema[T any]() any {
	// Structured Outputs uses a subset of JSON schema
	// These flags are necessary to comply with the subset
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	var v T
	schema := reflector.Reflect(v)
	return schema
}

// TranslationResponseSchema is the JSON schema for the translation response
var TranslationResponseSchema = GenerateSchema[TranslationResponse]()

// Translator handles subtitle translation operations
type Translator struct {
	client openai.Client
	config TranslationConfig
}

// NewTranslator creates a new Translator instance with the default configuration
func NewTranslator() *Translator {
	return &Translator{
		client: openai.NewClient(),
		config: DefaultTranslationConfig(),
	}
}

// NewTranslatorWithConfig creates a new Translator with a custom configuration
func NewTranslatorWithConfig(config TranslationConfig) *Translator {
	return &Translator{
		client: openai.NewClient(),
		config: config,
	}
}

// SetConfig updates the translator configuration
func (t *Translator) SetConfig(config TranslationConfig) {
	t.config = config
}

// TranslateSubtitleFile translates subtitles from a file path
func (t *Translator) TranslateSubtitleFile(inputPath, outputPath string) error {
	// Load subtitle file for translation
	subs, err := astisub.OpenFile(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open subtitle file: %w", err)
	}

	// If output path is empty, derive it from the input path
	if outputPath == "" {
		outputPath = deriveOutputPath(inputPath)
	}

	// Translate the subtitles
	err = t.TranslateSubtitles(subs)
	if err != nil {
		return fmt.Errorf("failed to translate subtitles: %w", err)
	}

	// Save the translated subtitles to the output file
	err = subs.Write(outputPath)
	if err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	fmt.Printf("Translated subtitles saved to %s\n", outputPath)
	return nil
}

// TranslateSubtitles translates the contents of an astisub.Subtitles object
func (t *Translator) TranslateSubtitles(subs *astisub.Subtitles) error {
	batchSize := t.config.BatchSize
	concurrencyLimit := t.config.ConcurrencyLimit
	batchCount := int(math.Ceil(float64(len(subs.Items)) / float64(batchSize)))
	
	// Create a semaphore to limit concurrency
	semaphore := make(chan struct{}, concurrencyLimit)
	var wg sync.WaitGroup
	
	// Channel to collect results from all goroutines
	translationResultsChan := make(chan []Subtitle, batchCount+1)
	
	// Process each batch in a separate goroutine
	for i := 0; i < batchCount; i++ {
		semaphore <- struct{}{}
		fmt.Printf("Batch %d / %d\n", i+1, batchCount)
		
		start := i * batchSize
		end := min(start+batchSize, len(subs.Items))
		batch := subs.Items[start:end]
		
		wg.Add(1)
		go func(batch []*astisub.Item) {
			defer wg.Done()
			defer func() { <-semaphore }()
			
			translated, err := t.translateBatch(batch)
			if err != nil {
				fmt.Printf("Error translating batch: %v\n", err)
				return
			}
			translationResultsChan <- translated
		}(batch)
	}

	// Wait for all goroutines to finish and close the results channel
	go func() {
		wg.Wait()
		close(translationResultsChan)
	}()
	
	// Collect and sort all translation results
	var allTranslations []Subtitle
	for translations := range translationResultsChan {
		allTranslations = append(allTranslations, translations...)
	}
	
	// Sort translations by index
	sort.Slice(allTranslations, func(i, j int) bool {
		return allTranslations[i].Index < allTranslations[j].Index
	})
	
	// Apply translations to original subtitles
	for _, sub := range subs.Items {
		for _, translation := range allTranslations {
			if sub.Index == translation.Index {
				for i, line := range sub.Lines {
					if i < len(translation.Lines) {
						for j := range line.Items {
							if j < len(translation.Lines[i].Items) {
								sub.Lines[i].Items[j].Text = translation.Lines[i].Items[j].Text
							}
						}
					}
				}
				break
			}
		}
	}
	
	return nil
}

// translateBatch translates a batch of subtitle items
func (t *Translator) translateBatch(subs []*astisub.Item) ([]Subtitle, error) {
	// Convert the subtitles to the desired format
	var subtitles []Subtitle
	for _, item := range subs {
		var lines []Line
		for _, line := range item.Lines {
			var items []LineItem
			for _, item := range line.Items {
				items = append(items, LineItem{
					Text: item.Text,
				})
			}
			lines = append(lines, Line{
				Items: items,
			})
		}

		subtitles = append(subtitles, Subtitle{
			Index: item.Index,
			Lines: lines,
		})
	}

	// Marshal the subtitles to JSON
	jsonData, err := json.Marshal(subtitles)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal subtitles: %w", err)
	}
	
	// Configure the JSON schema for the response
	schemaParam := openai.ResponseFormatJSONSchemaJSONSchemaParam{
		Name:        "subtitles",
		Description: openai.String("Translated subtitles"),
		Schema:      TranslationResponseSchema,
		Strict:      openai.Bool(true),
	}
	
	// Prepare the system message based on target language
	systemMessage := fmt.Sprintf("Translate subtitles to %s", t.config.TargetLanguage)
	
	// Call the OpenAI API for translation
	response, err := t.client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(systemMessage),
			openai.UserMessage(string(jsonData)),
		},
		ResponseFormat: openai.ChatCompletionNewParamsResponseFormatUnion{
			OfJSONSchema: &openai.ResponseFormatJSONSchemaParam{
				JSONSchema: schemaParam,
			},
		},
		Model: t.config.Model,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to call translation API: %w", err)
	}

	// Unmarshal the response
	var translationResponse TranslationResponse
	err = json.Unmarshal([]byte(response.Choices[0].Message.Content), &translationResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal translation response: %w", err)
	}

	return translationResponse.Subtitles, nil
}

// deriveOutputPath creates an output path in the same directory as the input file
// with language segments (eng, eng.hi, en, en.hi) replaced by 'pl'
// If no language segment is found, it adds 'pl' before the extension
func deriveOutputPath(inputPath string) string {
	// Find the last dot for extension
	lastDotIndex := -1
	for i := len(inputPath) - 1; i >= 0; i-- {
		if inputPath[i] == '.' {
			lastDotIndex = i
			break
		}
	}

	if lastDotIndex == -1 {
		// No extension found, just append .pl
		return inputPath + ".pl"
	}

	// Get base and extension
	basePath := inputPath[:lastDotIndex]
	extension := inputPath[lastDotIndex:]

	// Check for common suffixes that indicate language codes
	// First check for multi-part language codes with .hi suffix (highest priority)
	if hasAnySuffix(basePath, ".eng.hi", ".en.hi") {
		// Find the last occurrence of either suffix
		idx := max(lastIndexOf(basePath, ".eng.hi"), lastIndexOf(basePath, ".en.hi"))
		return basePath[:idx] + ".pl" + extension
	}

	if hasAnySuffix(basePath, "_eng.hi", "_en.hi") {
		// Find the last occurrence of either suffix
		idx := max(lastIndexOf(basePath, "_eng.hi"), lastIndexOf(basePath, "_en.hi"))
		return basePath[:idx] + "_pl" + extension
	}

	// Then check for simple language codes
	if hasAnySuffix(basePath, ".eng", ".en") {
		// Find the last occurrence of either suffix
		idx := max(lastIndexOf(basePath, ".eng"), lastIndexOf(basePath, ".en"))
		return basePath[:idx] + ".pl" + extension
	}

	if hasAnySuffix(basePath, "_eng", "_en") {
		// Find the last occurrence of either suffix
		idx := max(lastIndexOf(basePath, "_eng"), lastIndexOf(basePath, "_en"))
		return basePath[:idx] + "_pl" + extension
	}

	// No language segment found, add "pl" as the last segment
	return basePath + ".pl" + extension
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// lastIndexOf returns the index of the last occurrence of substr in s,
// or -1 if substr is not present in s
func lastIndexOf(s, substr string) int {
	for i := len(s) - len(substr); i >= 0; i-- {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// hasAnySuffix returns true if s ends with any of the provided suffixes
func hasAnySuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
			return true
		}
	}
	return false
}