// Package main provides the command-line interface entry point for the downloader tool,
// parsing CLI flags and routing subcommands for downloading, resuming, listing, and inspecting sessions.
package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"downloader/internal/engine"
	"downloader/internal/model"
	"downloader/internal/store"
)

// main is the CLI entry point dispatching subcommands or launching downloads.
func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "list":
			handleList()
			return
		case "resume":
			handleResume(os.Args[2:])
			return
		case "status":
			handleStatus(os.Args[2:])
			return
		}
	}

	handleDownload()
}

// handleList displays a formatted table of all saved sessions and their progress.
func handleList() {
	st, err := store.DefaultStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error accessing storage: %v\n", err)
		os.Exit(1)
	}

	sessions, err := st.ListSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing sessions: %v\n", err)
		os.Exit(1)
	}

	if len(sessions) == 0 {
		fmt.Println("No saved sessions found.")
		return
	}

	fmt.Printf("%-20s %-10s %-12s %-20s\n", "SESSION NAME", "FILES", "STATUS", "LAST UPDATED")
	fmt.Println(strings.Repeat("-", 65))
	for _, s := range sessions {
		completed := 0
		for _, j := range s.Jobs {
			if j.Status == model.StatusCompleted {
				completed++
			}
		}
		status := fmt.Sprintf("%d/%d Done", completed, len(s.Jobs))
		fmt.Printf("%-20s %-10d %-12s %-20s\n",
			s.Name, len(s.Jobs), status, s.UpdatedAt.Format("2006-01-02 15:04:05"))
	}
}

// handleStatus prints detailed progress, file statuses, and retry counts for a given session.
func handleStatus(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: downloader status <session-name>")
		return
	}
	name := args[0]
	st, err := store.DefaultStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sess, err := st.LoadSession(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Session: %s\nOutput Directory: %s\nUpdated: %s\n\n",
		sess.Name, sess.OutputDir, sess.UpdatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("%-30s %-12s %-15s %-10s\n", "FILE", "STATUS", "PROGRESS", "RETRIES")
	fmt.Println(strings.Repeat("-", 70))
	for _, j := range sess.Jobs {
		progress := fmt.Sprintf("%s / %s", model.FormatBytes(j.DownloadedBytes), model.FormatBytes(j.TotalSize))
		fmt.Printf("%-30s %-12s %-15s %-10d\n",
			j.ResolvedFilename, j.Status, progress, j.RetryCount)
	}
}

// handleResume resumes a previously interrupted or paused download session.
func handleResume(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: downloader resume <session-name> [-workers N] [-limit X]")
		return
	}
	name := args[0]
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	workers := fs.Int("workers", 3, "Concurrent worker pool size")
	limitStr := fs.String("limit", "", "Bandwidth limit (e.g. 5MB/s, 500KB/s)")
	_ = fs.Parse(args[1:])

	st, err := store.DefaultStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	sess, err := st.LoadSession(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading session: %v\n", err)
		os.Exit(1)
	}

	cfg := engine.Config{
		SessionName:  sess.Name,
		OutputDir:    sess.OutputDir,
		WorkerCount:  *workers,
		RateLimitBps: parseRateLimit(*limitStr),
	}

	eng := engine.NewEngine(cfg, st)
	if err := eng.Run(context.Background(), sess); err != nil {
		fmt.Fprintf(os.Stderr, "Run error: %v\n", err)
	}
}

// handleDownload parses flags for a new download run, sets up the session, and triggers the engine.
func handleDownload() {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	file := fs.String("file", "", "Path to file containing list of URLs")
	name := fs.String("name", "", "Session/Job name")
	workers := fs.Int("workers", 3, "Number of concurrent workers (default 3)")
	limitStr := fs.String("limit", "", "Bandwidth limit (e.g. 5MB/s, 1000KB/s)")
	sha256Expected := fs.String("sha256", "", "Expected SHA-256 hash (for single-file verification)")
	dir := fs.String("dir", ".", "Destination directory")

	_ = fs.Parse(os.Args[1:])

	if *file == "" {
		fmt.Println("Usage: downloader -file urls.txt [-name jobname] [-workers 3] [-limit 5MB/s] [-dir ./downloads]")
		fs.PrintDefaults()
		return
	}

	sessionName := *name
	if sessionName == "" {
		sessionName = fmt.Sprintf("job_%d", time.Now().Unix())
	}

	st, err := store.DefaultStore()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	jobs, err := parseURLFile(*file, *sha256Expected)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading URL file: %v\n", err)
		os.Exit(1)
	}

	sess := &model.Session{
		Name:      sessionName,
		OutputDir: *dir,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Jobs:      jobs,
	}

	cfg := engine.Config{
		SessionName:  sessionName,
		OutputDir:    *dir,
		WorkerCount:  *workers,
		RateLimitBps: parseRateLimit(*limitStr),
	}

	eng := engine.NewEngine(cfg, st)
	if err := eng.Run(context.Background(), sess); err != nil {
		fmt.Fprintf(os.Stderr, "Download error: %v\n", err)
	}
}

// parseURLFile parses URLs and optional SHA-256 hashes from a newline-delimited text file.
func parseURLFile(filePath, singleSHA string) ([]*model.FileJob, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var jobs []*model.FileJob
	scanner := bufio.NewScanner(f)
	idx := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Fields(line)
		u := parts[0]
		expectedHash := singleSHA
		if len(parts) > 1 {
			expectedHash = parts[1] // Allow format: <url> <sha256>
		}

		hash := sha256.Sum256([]byte(u))
		jobID := hex.EncodeToString(hash[:8])

		job := &model.FileJob{
			ID:             fmt.Sprintf("%s_%d", jobID, idx),
			URL:            u,
			TotalSize:      -1,
			Status:         model.StatusPending,
			ExpectedSHA256: expectedHash,
		}
		jobs = append(jobs, job)
		idx++
	}

	return jobs, scanner.Err()
}

// parseRateLimit parses human-readable bandwidth strings (e.g., "5MB/s", "500KB/s") into bytes per second.
func parseRateLimit(s string) int64 {
	s = strings.ToUpper(strings.TrimSpace(s))
	if s == "" {
		return 0
	}

	s = strings.TrimSuffix(s, "/S")
	s = strings.TrimSuffix(s, "S")

	multiplier := int64(1)
	if strings.HasSuffix(s, "GB") || strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimRight(s, "GB")
	} else if strings.HasSuffix(s, "MB") || strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = strings.TrimRight(s, "MB")
	} else if strings.HasSuffix(s, "KB") || strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = strings.TrimRight(s, "KB")
	} else if strings.HasSuffix(s, "B") {
		multiplier = 1
		s = strings.TrimRight(s, "B")
	}

	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return int64(val * float64(multiplier))
}
