package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/ikhsan3adi/gemini-web2api/internal/format"
	"github.com/ikhsan3adi/gemini-web2api/internal/gemini"
	"github.com/ikhsan3adi/gemini-web2api/internal/models"
	"github.com/ikhsan3adi/gemini-web2api/internal/multimodal"
)

type imageRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	ResponseFormat string `json:"response_format"`
}

func (a *App) handleImageGenerations(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil || len(body) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid JSON"}})
		return
	}
	var req imageRequest
	if err := json.Unmarshal(body, &req); err != nil || strings.TrimSpace(req.Prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "prompt is required"}})
		return
	}
	a.writeImages(w, req.Model, req.Prompt, nil)
}

func (a *App) handleImageEdits(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "invalid multipart body"}})
		return
	}
	prompt := r.FormValue("prompt")
	if strings.TrimSpace(prompt) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "prompt is required"}})
		return
	}
	file, hdr, err := r.FormFile("image")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "image file is required"}})
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 20<<20))
	if err != nil || len(data) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": "empty image"}})
		return
	}
	name := "image"
	if hdr != nil && hdr.Filename != "" {
		name = hdr.Filename
	}
	a.writeImages(w, r.FormValue("model"), prompt, []format.Image{{
		Data: data,
		MIME: http.DetectContentType(data),
		Name: name,
	}})
}

func (a *App) writeImages(w http.ResponseWriter, modelName, prompt string, images []format.Image) {
	if modelName == "" || modelName == "gemini-3.1-flash-image" {
		modelName = a.Cfg.DefaultModel
		if modelName == "" {
			modelName = models.DefaultModelName
		}
	}
	resolved, err := models.Resolve(modelName, a.Cfg.DefaultModel)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	var refs []gemini.FileRef
	if len(images) > 0 {
		refs, err = a.uploadImages(images)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]any{"message": err.Error()}})
			return
		}
	}
	turn, err := a.Gem.GenerateTurn(prompt, resolved.Mode, resolved.Think, refs, resolved.Extra)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]any{"message": err.Error()}})
		return
	}
	if turn.ServedModel != "" {
		w.Header().Set("X-Gemini-Served-Model", turn.ServedModel)
	}
	if len(turn.Media) == 0 {
		msg := "Gemini returned no generated image"
		if turn.Text != "" {
			msg = turn.Text
		}
		writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]any{"message": msg, "served_model": turn.ServedModel}})
		return
	}
	cookie := ""
	if info, err := a.Gem.Cookies.Load(); err == nil {
		cookie = info.Cookie
	}
	var requester gemini.Requester = a.Gem.HTTP
	if requester == nil {
		requester = a.HTTPClient
	}
	var data []map[string]any
	for _, media := range turn.Media {
		blob, mime, err := multimodal.DownloadGenerated(requester, cookie, media.URL)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]any{"error": map[string]any{"message": err.Error()}})
			return
		}
		if mime == "" {
			mime = media.MIME
		}
		data = append(data, map[string]any{
			"b64_json":     base64.StdEncoding.EncodeToString(blob),
			"mime_type":    mime,
			"served_model": turn.ServedModel,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"created": 0, "data": data})
}
