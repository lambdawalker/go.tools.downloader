package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"downloader/internal/auth"
	"downloader/internal/model"
)

func TestGrabDownloadSuccessWithAuth(t *testing.T) {
	content := "Hello Grab Resumable Streaming with Auth!"
	h := sha256.Sum256([]byte(content))
	expectedHash := hex.EncodeToString(h[:])

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-grab-token" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(content)))
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "transfer_grab_test_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDownloader(nil)
	job := &model.FileJob{
		ID:               "job-1",
		URL:              server.URL + "/test.txt",
		ResolvedFilename: "test.txt",
		ExpectedSHA256:   expectedHash,
	}

	strat := auth.NewBearerStrategy("test-grab-token")
	err = d.Download(context.Background(), job, tmpDir, strat)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}

	if job.Status != model.StatusCompleted {
		t.Fatalf("expected StatusCompleted, got %s", job.Status)
	}

	targetPath := filepath.Join(tmpDir, "test.txt")
	data, err := os.ReadFile(targetPath)
	if err != nil {
		t.Fatalf("failed reading downloaded file: %v", err)
	}
	if string(data) != content {
		t.Fatalf("content mismatch: got %q, expected %q", string(data), content)
	}
	if job.ActualSHA256 != expectedHash {
		t.Fatalf("sha256 mismatch: got %s, expected %s", job.ActualSHA256, expectedHash)
	}
}

func TestGrabDownloadAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "transfer_grab_err_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDownloader(nil)
	job := &model.FileJob{
		ID:               "job-fail",
		URL:              server.URL + "/unauth.txt",
		ResolvedFilename: "unauth.txt",
	}

	err = d.Download(context.Background(), job, tmpDir, auth.NewAnonymousStrategy())
	if err == nil {
		t.Fatalf("expected auth error, got nil")
	}

	if !auth.IsAuthError(err) {
		t.Fatalf("expected error to be an *auth.AuthError, got: %T (%v)", err, err)
	}
	if !auth.IsUnauthorized(err) {
		t.Fatalf("expected error to be Unauthorized, got: %v", err)
	}
}

func TestGrabDownloadRateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "45")
		http.Error(w, "Rate Limited", http.StatusTooManyRequests)
	}))
	defer server.Close()

	tmpDir, err := os.MkdirTemp("", "transfer_grab_rate_*")
	if err != nil {
		t.Fatalf("failed creating temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	d := NewDownloader(nil)
	job := &model.FileJob{
		ID:               "job-rate",
		URL:              server.URL + "/rate.txt",
		ResolvedFilename: "rate.txt",
	}

	err = d.Download(context.Background(), job, tmpDir, nil)
	if err == nil {
		t.Fatalf("expected rate limit error, got nil")
	}

	if !auth.IsRateLimited(err) {
		t.Fatalf("expected error to be RateLimited, got: %v", err)
	}
	authErr, _ := auth.AsAuthError(err)
	if authErr.RetryAfter != 45*time.Second {
		t.Fatalf("expected RetryAfter 45s, got %s", authErr.RetryAfter)
	}
}
