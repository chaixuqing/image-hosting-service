package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

var png1x1 = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
	0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41,
	0x54, 0x08, 0xd7, 0x63, 0xf8, 0xff, 0xff, 0x3f,
	0x00, 0x05, 0xfe, 0x02, 0xfe, 0xdc, 0xcc, 0x59,
	0xe7, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e,
	0x44, 0xae, 0x42, 0x60, 0x82,
}

func TestImageLifecycle(t *testing.T) {
	mux := newMux(t.TempDir())

	uploadReq := newUploadRequest(t, png1x1, "tiny.png")
	uploadResp := httptest.NewRecorder()
	mux.ServeHTTP(uploadResp, uploadReq)
	if uploadResp.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", uploadResp.Code, uploadResp.Body.String())
	}

	var payload struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(uploadResp.Body).Decode(&payload); err != nil {
		t.Fatalf("failed to parse upload response: %v", err)
	}
	if payload.ID == "" {
		t.Fatal("expected non-empty image id")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/images/"+payload.ID, nil)
	getResp := httptest.NewRecorder()
	mux.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", getResp.Code, getResp.Body.String())
	}
	if !bytes.Equal(getResp.Body.Bytes(), png1x1) {
		t.Fatal("downloaded image content did not match upload")
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/images/"+payload.ID, nil)
	deleteResp := httptest.NewRecorder()
	mux.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}

	getMissingReq := httptest.NewRequest(http.MethodGet, "/images/"+payload.ID, nil)
	getMissingResp := httptest.NewRecorder()
	mux.ServeHTTP(getMissingResp, getMissingReq)
	if getMissingResp.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", getMissingResp.Code)
	}
}

func TestRejectsNonImageUpload(t *testing.T) {
	mux := newMux(t.TempDir())

	uploadReq := newUploadRequest(t, []byte("plain text"), "note.txt")
	uploadResp := httptest.NewRecorder()
	mux.ServeHTTP(uploadResp, uploadReq)

	if uploadResp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", uploadResp.Code)
	}
}

func newUploadRequest(t *testing.T, content []byte, fileName string) *http.Request {
	t.Helper()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("image", fileName)
	if err != nil {
		t.Fatalf("failed creating form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(content)); err != nil {
		t.Fatalf("failed writing form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("failed closing writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/images", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}
