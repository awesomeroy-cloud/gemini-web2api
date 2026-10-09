package gemini

import (
	"encoding/json"
	"strings"
)

// Media is one generated file carried in a StreamGenerate candidate.
type Media struct {
	URL  string
	MIME string
	Name string
}

// Turn is the text, generated media, and the model name Gemini put in the payload.
type Turn struct {
	Text        string
	Media       []Media
	ServedModel string
}

// ExtractTurn reads the longest candidate text, gg-dl media, and payload model name.
func ExtractTurn(raw string) (Turn, error) {
	if code, ok := IsBardError(raw); ok {
		return Turn{}, errBard(code)
	}
	var turn Turn
	for _, line := range strings.Split(raw, "\n") {
		text, media, served := extractLine(line)
		if len(text) > len(turn.Text) {
			turn.Text = text
		}
		turn.Media = appendNewMedia(turn.Media, media)
		if served != "" {
			turn.ServedModel = served
		}
	}
	turn.Text = CleanText(turn.Text, true)
	return turn, nil
}

func errBard(code string) error {
	return &bardError{code: code}
}

type bardError struct{ code string }

func (e *bardError) Error() string {
	return "Gemini upstream rejected request: BardErrorInfo [" + e.code + "]"
}

func extractLine(line string) (string, []Media, string) {
	var arr []any
	if err := json.Unmarshal([]byte(line), &arr); err != nil || len(arr) == 0 {
		return "", nil, ""
	}
	first, ok := arr[0].([]any)
	if !ok || len(first) < 3 {
		return "", nil, ""
	}
	innerStr, ok := first[2].(string)
	if !ok || len(innerStr) < 20 {
		return "", nil, ""
	}
	var inner []any
	if err := json.Unmarshal([]byte(innerStr), &inner); err != nil {
		return "", nil, ""
	}
	served, _ := at(inner, 42).(string)
	parts, _ := at(inner, 4).([]any)
	var text string
	var media []Media
	for _, part := range parts {
		cand, ok := part.([]any)
		if !ok {
			continue
		}
		if t := candidateText(cand); len(t) > len(text) {
			text = t
		}
		media = append(media, candidateMedia(cand)...)
	}
	return text, media, served
}

func candidateText(cand []any) string {
	list, ok := at(cand, 1).([]any)
	if !ok || len(list) == 0 {
		return ""
	}
	s, _ := list[0].(string)
	return s
}

func candidateMedia(cand []any) []Media {
	rich := at(cand, 12)
	var items []any
	switch v := rich.(type) {
	case []any:
		if boxed, ok := at(v, 7).([]any); ok {
			items = boxed
		}
	case map[string]any:
		if boxed, ok := v["8"].([]any); ok {
			items = boxed
		}
	}
	var out []Media
	for _, item := range items {
		row, ok := item.([]any)
		if !ok {
			continue
		}
		meta, _ := at(at(row, 0).([]any), 3).([]any)
		if meta == nil {
			// item[0][0][3] is the shape in the 2026-08-24 capture
			if wrap, ok := at(row, 0).([]any); ok && len(wrap) > 0 {
				if inner, ok := wrap[0].([]any); ok {
					meta, _ = at(inner, 3).([]any)
				}
			}
		}
		if meta == nil {
			continue
		}
		u, _ := at(meta, 3).(string)
		if u == "" || strings.Contains(u, "image_generation_content") {
			continue
		}
		if !strings.Contains(u, "googleusercontent.com/gg-dl/") && !strings.Contains(u, "usercontent.google.com/rd-gg-dl/") {
			continue
		}
		name, _ := at(meta, 2).(string)
		mime, _ := at(meta, 11).(string)
		if mime == "" {
			mime = "image/jpeg"
		}
		out = append(out, Media{URL: u, MIME: mime, Name: name})
	}
	return out
}

func at(v any, i int) any {
	list, ok := v.([]any)
	if !ok || i < 0 || i >= len(list) {
		return nil
	}
	return list[i]
}

func appendNewMedia(dst, src []Media) []Media {
	for _, m := range src {
		seen := false
		for _, d := range dst {
			if d.URL == m.URL {
				seen = true
				break
			}
		}
		if !seen {
			dst = append(dst, m)
		}
	}
	return dst
}
