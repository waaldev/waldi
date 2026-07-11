package web

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"waldi/internal/storage"
)

const maxImageUploadBytes = 5 << 20

func (s *Server) handleImageUpload(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r)
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !s.requireVerified(w, r, user) {
		return
	}
	if s.images == nil {
		http.Error(w, "uploads unavailable", http.StatusServiceUnavailable)
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxImageUploadBytes)
	if err := r.ParseMultipartForm(maxImageUploadBytes); err != nil {
		http.Error(w, "image too large", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "image is required", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	data, err := storage.ProcessImage(file)
	if err != nil {
		http.Error(w, "invalid image", http.StatusBadRequest)
		return
	}

	name, err := randomUploadName(".webp")
	if err != nil {
		s.logger.Error("creating upload name", "err", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	publicURL, err := s.images.Save(r.Context(), user.Username, name, data, "image/webp")
	if err != nil {
		s.logger.Error("saving upload", "err", err)
		http.Error(w, "upload failed", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"url": publicURL})
}

func (s *Server) handleMedia(w http.ResponseWriter, r *http.Request) {
	if s.s3media == nil {
		http.NotFound(w, r)
		return
	}
	key := r.PathValue("key")
	if key == "" {
		http.NotFound(w, r)
		return
	}
	if err := s.s3media.Stream(r.Context(), w, key); err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		s.logger.Error("serving media", "key", key, "err", err)
		http.Error(w, "media unavailable", http.StatusInternalServerError)
	}
}

func randomUploadName(ext string) (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]) + ext, nil
}
