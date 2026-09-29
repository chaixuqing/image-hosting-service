package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultStorageDir = "uploads"
	maxUploadBytes    = 10 << 20 // 10MB
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	storageDir := os.Getenv("IMAGE_STORAGE_DIR")
	if storageDir == "" {
		storageDir = defaultStorageDir
	}

	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		log.Fatalf("failed to create storage directory: %v", err)
	}

	addr := ":" + port
	log.Printf("image hosting service listening on %s", addr)
	if err := http.ListenAndServe(addr, newMux(storageDir)); err != nil {
		log.Fatal(err)
	}
}

func newMux(storageDir string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/images", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		uploadImage(w, r, storageDir)
	})
	mux.HandleFunc("/images/", func(w http.ResponseWriter, r *http.Request) {
		id, err := extractID(r.URL.Path)
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		switch r.Method {
		case http.MethodGet:
			serveImage(w, r, storageDir, id)
		case http.MethodDelete:
			deleteImage(w, storageDir, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	return mux
}

func uploadImage(w http.ResponseWriter, r *http.Request, storageDir string) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "missing image form field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	firstChunk := make([]byte, 512)
	n, err := file.Read(firstChunk)
	if err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "failed to read uploaded image", http.StatusBadRequest)
		return
	}
	contentType := http.DetectContentType(firstChunk[:n])
	if !strings.HasPrefix(contentType, "image/") {
		http.Error(w, "uploaded file is not an image", http.StatusBadRequest)
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext == "" {
		extensions, _ := mime.ExtensionsByType(contentType)
		if len(extensions) > 0 {
			ext = extensions[0]
		} else {
			ext = ".img"
		}
	}

	id, err := randomID()
	if err != nil {
		http.Error(w, "failed to generate id", http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(storageDir, id+ext)
	out, err := os.OpenFile(filePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		http.Error(w, "failed to store image", http.StatusInternalServerError)
		return
	}
	defer out.Close()

	if _, err := out.Write(firstChunk[:n]); err != nil {
		http.Error(w, "failed to store image", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		http.Error(w, "failed to store image", http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"id":  id,
		"url": "/images/" + id,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(response)
}

func serveImage(w http.ResponseWriter, r *http.Request, storageDir, id string) {
	imagePath, err := findImageByID(storageDir, id)
	if err != nil {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, imagePath)
}

func deleteImage(w http.ResponseWriter, storageDir, id string) {
	imagePath, err := findImageByID(storageDir, id)
	if err != nil {
		http.Error(w, "image not found", http.StatusNotFound)
		return
	}
	if err := os.Remove(imagePath); err != nil {
		http.Error(w, "failed to delete image", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func findImageByID(storageDir, id string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(storageDir, id+".*"))
	if err != nil || len(matches) == 0 {
		return "", errors.New("not found")
	}
	return matches[0], nil
}

func extractID(path string) (string, error) {
	if !strings.HasPrefix(path, "/images/") {
		return "", errors.New("invalid path")
	}
	id := strings.TrimPrefix(path, "/images/")
	if id == "" || strings.Contains(id, "/") || strings.Contains(id, "..") {
		return "", errors.New("invalid id")
	}
	return id, nil
}

func randomID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
