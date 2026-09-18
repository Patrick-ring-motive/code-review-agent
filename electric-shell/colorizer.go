package main

import (
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const reset = "\x1b[0m"

type rgb struct {
	red   int
	green int
	blue  int
}

type rule struct {
	pattern *regexp.Regexp
	color   rgb
	dynamic bool
}

type highlight struct {
	start int
	end   int
	color rgb
}

var rules = []rule{
	{regexp.MustCompile(`[\*•]+`), rgb{100, 149, 237}, false},
	{regexp.MustCompile(`[{}‘’']+`), rgb{255, 121, 198}, false},
	{regexp.MustCompile(`[\[\]“”"]+`), rgb{186, 125, 255}, false},
	{regexp.MustCompile(`[()]+`), rgb{255, 165, 0}, false},
	{regexp.MustCompile(`[<>]+`), rgb{255, 165, 0}, false},
	{regexp.MustCompile(`[0-9]+`), rgb{0, 191, 255}, false},
	{nil, rgb{}, true},
	{regexp.MustCompile(`(?i)\b(Y|Yellows?|banana[a-z]*|lemons?|corn|maize)\b`), rgb{255, 255, 0}, false},
	{regexp.MustCompile(`(?i)\b(R|Reds?|Apple[a-z]*|tomatoe?s?|strawberry|strawberries)\b`), rgb{255, 0, 0}, false},
	{regexp.MustCompile(`(?i)\b(G|Green[a-z]*|plants?|trees?|leaf|limes?|lettuce|vegetables?)\b`), rgb{0, 255, 0}, false},
	{regexp.MustCompile(`(?i)(\\b|\b(B|Blue[a-z]*)\b)`), rgb{78, 78, 255}, false},
	{regexp.MustCompile(`(?i)\b(O|Oranges?|pumpkins?)\b`), rgb{255, 165, 0}, false},
	{regexp.MustCompile(`(?i)\b(P|Pinks?|Magentas?)\b`), rgb{255, 105, 180}, false},
	{regexp.MustCompile(`(?i)\b(V|Violets?|Purples?|grapes?|eggplants?)\b`), rgb{186, 125, 255}, false},
	{regexp.MustCompile(`(?i)\b(W|Whites?)\b`), rgb{255, 255, 255}, false},
	{regexp.MustCompile(`(?i)\bx\b`), rgb{255, 255, 255}, false},
}

func hslColor(hue float64) rgb {
	chroma := 1.0
	sector := hue / 60
	second := chroma * (1 - math.Abs(math.Mod(sector, 2)-1))
	values := [][3]float64{
		{chroma, second, 0},
		{second, chroma, 0},
		{0, chroma, second},
		{0, second, chroma},
		{second, 0, chroma},
		{chroma, 0, second},
	}
	value := values[int(sector)%len(values)]
	return rgb{
		int(math.Round(value[0] * 255)),
		int(math.Round(value[1] * 255)),
		int(math.Round(value[2] * 255)),
	}
}

func symbolRanges(text string) [][]int {
	var ranges [][]int
	start := -1
	for index := 0; index < len(text); {
		character, size := utf8.DecodeRuneInString(text[index:])
		excluded := unicode.IsSpace(character) ||
			(character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			strings.ContainsRune("*•{}‘’'[]“”\"()<>", character) ||
			(character == '\\' && index+size < len(text) && text[index+size] == 'b')
		if !excluded && start < 0 {
			start = index
		}
		if excluded && start >= 0 {
			ranges = append(ranges, []int{start, index})
			start = -1
		}
		index += size
	}
	if start >= 0 {
		ranges = append(ranges, []int{start, len(text)})
	}
	return ranges
}

func colorize(text string, enabled bool) string {
	if !enabled || text == "" {
		return text
	}

	occupied := make([]bool, len(text))
	var highlights []highlight
	for _, currentRule := range rules {
		var matches [][]int
		if currentRule.dynamic {
			matches = symbolRanges(text)
		} else {
			matches = currentRule.pattern.FindAllStringIndex(text, -1)
		}
		for _, match := range matches {
			overlaps := false
			for _, used := range occupied[match[0]:match[1]] {
				if used {
					overlaps = true
					break
				}
			}
			if overlaps {
				continue
			}
			for index := match[0]; index < match[1]; index++ {
				occupied[index] = true
			}
			color := currentRule.color
			if currentRule.dynamic {
				character, _ := utf8.DecodeRuneInString(text[match[0]:match[1]])
				rotation := (int(character) * 17) % 360
				color = hslColor(float64((120 + rotation) % 360))
			}
			highlights = append(highlights, highlight{match[0], match[1], color})
		}
	}

	sort.Slice(highlights, func(left, right int) bool {
		return highlights[left].start < highlights[right].start
	})
	var result strings.Builder
	cursor := 0
	for _, match := range highlights {
		result.WriteString(text[cursor:match.start])
		fmt.Fprintf(
			&result,
			"\x1b[38;2;%d;%d;%dm%s%s",
			match.color.red,
			match.color.green,
			match.color.blue,
			text[match.start:match.end],
			reset,
		)
		cursor = match.end
	}
	result.WriteString(text[cursor:])
	return result.String()
}

type colorWriter struct {
	destination io.Writer
	enabled     bool
	pending     []byte
}

func newColorWriter(destination io.Writer, enabled bool) *colorWriter {
	return &colorWriter{destination: destination, enabled: enabled}
}

func (writer *colorWriter) writePlain(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	_, err := io.WriteString(writer.destination, colorize(string(data), writer.enabled))
	return err
}

func escapeEnd(data []byte, start int) (int, bool) {
	if start+1 >= len(data) {
		return 0, false
	}
	switch data[start+1] {
	case '[':
		for index := start + 2; index < len(data); index++ {
			if data[index] >= '@' && data[index] <= '~' {
				return index + 1, true
			}
		}
	case ']':
		for index := start + 2; index < len(data); index++ {
			if data[index] == '\a' {
				return index + 1, true
			}
			if data[index] == '\x1b' && index+1 < len(data) && data[index+1] == '\\' {
				return index + 2, true
			}
		}
	default:
		return start + 2, true
	}
	return 0, false
}

func (writer *colorWriter) Write(input []byte) (int, error) {
	data := append(writer.pending, input...)
	writer.pending = nil
	cursor := 0
	for cursor < len(data) {
		offset := strings.IndexByte(string(data[cursor:]), '\x1b')
		if offset < 0 {
			if err := writer.writePlain(data[cursor:]); err != nil {
				return 0, err
			}
			break
		}
		escapeStart := cursor + offset
		if err := writer.writePlain(data[cursor:escapeStart]); err != nil {
			return 0, err
		}
		end, complete := escapeEnd(data, escapeStart)
		if !complete {
			writer.pending = append(writer.pending, data[escapeStart:]...)
			break
		}
		sequence := data[escapeStart:end]
		if _, err := writer.destination.Write(sequence); err != nil {
			return 0, err
		}
		cursor = end
	}
	return len(input), nil
}

func (writer *colorWriter) flush() error {
	if len(writer.pending) == 0 {
		return nil
	}
	_, err := writer.destination.Write(writer.pending)
	writer.pending = nil
	return err
}
