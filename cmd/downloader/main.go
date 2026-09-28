// Package main provides the command-line interface entry point for the downloader tool,
// parsing CLI flags and routing subcommands for downloading, resuming, listing, inspecting sessions,
// and managing encrypted master-password credential vaults.
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

	"downloader/internal/auth"
	"downloader/internal/engine"
	"downloader/internal/model"
	"downloader/internal/store"
)

// printError prints formatted error output to stderr, ignoring write errors.
func printError(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

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
		case "auth":
			handleAuth(os.Args[2:])
			return
		}
	}

	handleDownload()
}

// handleList displays a formatted table of all saved sessions and their progress.
func handleList() {
	st, err := store.DefaultStore()
	if err != nil {
		printError("Error accessing storage: %v\n", err)
		os.Exit(1)
	}

	sessions, err := st.ListSessions()
	if err != nil {
		printError("Error listing sessions: %v\n", err)
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
		printError("Error: %v\n", err)
		os.Exit(1)
	}

	sess, err := st.LoadSession(name)
	if err != nil {
		printError("Error: %v\n", err)
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

// handleAuth inspects or configures persistent domain cache and master-password-encrypted vault.
func handleAuth(args []string) {
	vault, err := auth.DefaultVault()
	if err != nil {
		printError("Error accessing vault path: %v\n", err)
		os.Exit(1)
	}

	if len(args) == 0 || args[0] == "list" {
		printAuthOverview(vault)
		return
	}

	switch args[0] {
	case "vault":
		handleVaultSubcommand(vault, args[1:])
	case "profile":
		handleProfileSubcommand(vault, args[1:])
	case "domain":
		handleDomainSubcommand(vault, args[1:])
	default:
		printAuthHelp()
	}
}

// printAuthOverview lists entries from both the legacy domain cache and the encrypted vault.
func printAuthOverview(vault *auth.Vault) {
	fmt.Println("=== Encrypted Credential Vault ===")
	if !vault.Exists() {
		fmt.Println("Status: Vault not initialized. (Run `downloader auth vault init` to create one)")
	} else {
		fmt.Printf("Vault Path: %s\n", vault.Path())
		fmt.Println("Status: Vault exists (Encrypted AES-256-GCM with Master Password)")
		fmt.Println("Tip: Run `downloader auth profile list` to inspect profiles after unlocking.")
	}

	fmt.Println("\n=== Local Domain Cache (~/.downloader/auth) ===")
	authStore, err := auth.DefaultStore()
	if err == nil {
		entries := authStore.ListEntries()
		if len(entries) == 0 {
			fmt.Println("No domain cache entries found.")
		} else {
			fmt.Printf("%-30s %-12s %-30s\n", "DOMAIN", "METHOD", "DETAILS")
			fmt.Println(strings.Repeat("-", 75))
			for domain, entry := range entries {
				fmt.Printf("%-30s %-12s %-30s\n", domain, entry.Method, maskSecret(entry.Token))
			}
		}
	}
}

// handleVaultSubcommand routes vault initialization and password change commands.
func handleVaultSubcommand(vault *auth.Vault, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: downloader auth vault [init | passwd]")
		return
	}

	switch args[0] {
	case "init":
		if vault.Exists() {
			fmt.Println("[Vault] Vault already exists at:", vault.Path())
			return
		}
		pass, err := auth.ConfirmPassword("[Vault] Enter Master Password for new vault: ")
		if err != nil {
			printError("Error: %v\n", err)
			os.Exit(1)
		}
		if err := vault.Init(pass); err != nil {
			printError("Failed initializing vault: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[Success] Encrypted credential vault initialized successfully at:", vault.Path())

	case "passwd":
		if !vault.Exists() {
			fmt.Println("[Vault] Vault does not exist. Initialize it first via `downloader auth vault init`.")
			return
		}
		oldPass, err := auth.PromptPassword("[Vault] Enter current Master Password: ")
		if err != nil {
			printError("Error: %v\n", err)
			os.Exit(1)
		}
		if err := vault.Unlock(oldPass); err != nil {
			printError("Error unlocking vault: %v\n", err)
			os.Exit(1)
		}
		newPass, err := auth.ConfirmPassword("[Vault] Enter new Master Password: ")
		if err != nil {
			printError("Error: %v\n", err)
			os.Exit(1)
		}
		if err := vault.ChangePassword(newPass); err != nil {
			printError("Error changing password: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("[Success] Master Password successfully updated and vault re-encrypted.")

	default:
		fmt.Println("Usage: downloader auth vault [init | passwd]")
	}
}

// handleProfileSubcommand manages named credential profiles within the encrypted vault.
func handleProfileSubcommand(vault *auth.Vault, args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: downloader auth profile [list | set <name> | remove <name>]")
		return
	}

	ensureVaultUnlocked(vault)

	switch args[0] {
	case "list":
		profiles := vault.ListProfiles()
		if len(profiles) == 0 {
			fmt.Println("No profiles found in encrypted vault.")
			return
		}
		fmt.Printf("%-25s %-15s %-30s\n", "PROFILE NAME", "METHOD", "DETAILS")
		fmt.Println(strings.Repeat("-", 70))
		for _, name := range profiles {
			p, _ := vault.GetProfile(name)
			details := ""
			switch p.Method {
			case auth.MethodBearer:
				details = fmt.Sprintf("Token: %s", maskSecret(p.Token))
			case auth.MethodBasic:
				details = fmt.Sprintf("User: %s", p.Username)
			case auth.MethodHeader:
				details = fmt.Sprintf("%d headers configured", len(p.Headers))
			}
			fmt.Printf("%-25s %-15s %-30s\n", name, p.Method, details)
		}

	case "set":
		if len(args) < 2 {
			fmt.Println("Usage: downloader auth profile set <profile-name>")
			return
		}
		name := args[1]
		p, err := promptProfileConfiguration(name)
		if err != nil {
			printError("Configuration error: %v\n", err)
			os.Exit(1)
		}
		if err := vault.SetProfile(name, p); err != nil {
			printError("Failed saving profile: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("[Success] Profile %q encrypted and saved into vault.\n", name)

	case "remove", "delete":
		if len(args) < 2 {
			fmt.Println("Usage: downloader auth profile remove <profile-name>")
			return
		}
		name := args[1]
		if vault.DeleteProfile(name) {
			fmt.Printf("[Success] Profile %q removed from vault.\n", name)
		} else {
			fmt.Printf("Profile %q not found in vault.\n", name)
		}

	default:
		fmt.Println("Usage: downloader auth profile [list | set <name> | remove <name>]")
	}
}

// handleDomainSubcommand maps a domain name to a default profile in the vault.
func handleDomainSubcommand(vault *auth.Vault, args []string) {
	if len(args) < 3 || args[0] != "set" {
		fmt.Println("Usage: downloader auth domain set <domain> <profile-name>")
		return
	}
	ensureVaultUnlocked(vault)

	domain := args[1]
	profileName := args[2]

	if _, ok := vault.GetProfile(profileName); !ok {
		printError("Error: profile %q does not exist in vault. Create it first via `downloader auth profile set %s`\n", profileName, profileName)
		os.Exit(1)
	}

	if err := vault.SetDomainDefault(domain, profileName); err != nil {
		printError("Failed saving domain mapping: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[Success] Domain %q successfully mapped to profile %q in encrypted vault.\n", domain, profileName)
}

// ensureVaultUnlocked prompts for master password if vault is locked or initializes a new one.
func ensureVaultUnlocked(vault *auth.Vault) {
	if vault.IsUnlocked() {
		return
	}

	if !vault.Exists() {
		fmt.Println("[Vault] No vault found. Initializing a new encrypted credential vault.")
		pass, err := auth.ConfirmPassword("[Vault] Enter Master Password: ")
		if err != nil {
			printError("Error: %v\n", err)
			os.Exit(1)
		}
		if err := vault.Init(pass); err != nil {
			printError("Failed initializing vault: %v\n", err)
			os.Exit(1)
		}
		return
	}

	pass, err := auth.PromptPassword("[Vault] Enter Master Password: ")
	if err != nil {
		printError("Error reading password: %v\n", err)
		os.Exit(1)
	}
	if err := vault.Unlock(pass); err != nil {
		printError("Error unlocking vault: %v\n", err)
		os.Exit(1)
	}
}

// promptProfileConfiguration interactively queries the user for credentials for a named profile.
func promptProfileConfiguration(name string) (*auth.Profile, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("\nConfigure credentials for profile %q:\n", name)
	fmt.Println("1. Bearer Token")
	fmt.Println("2. Basic Auth (username & password)")
	fmt.Println("3. Custom HTTP Headers (API Key, Personal Access Token)")
	fmt.Print("Select method [1-3] (default 1): ")

	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)

	switch choice {
	case "2":
		fmt.Print("Enter Username: ")
		user, _ := reader.ReadString('\n')
		pass, err := auth.PromptPassword("Enter Password: ")
		if err != nil {
			return nil, err
		}
		return &auth.Profile{
			Method:   auth.MethodBasic,
			Username: strings.TrimSpace(user),
			Password: pass,
		}, nil

	case "3":
		headers := make(map[string]string)
		for {
			fmt.Print("Enter Header Name (or press Enter to finish): ")
			hName, _ := reader.ReadString('\n')
			hName = strings.TrimSpace(hName)
			if hName == "" {
				break
			}
			hVal, err := auth.PromptPassword(fmt.Sprintf("Enter Header Value for %q: ", hName))
			if err != nil {
				return nil, err
			}
			headers[hName] = hVal
		}
		if len(headers) == 0 {
			return nil, fmt.Errorf("at least one header must be specified")
		}
		return &auth.Profile{
			Method:  auth.MethodHeader,
			Headers: headers,
		}, nil

	default: // Bearer
		token, err := auth.PromptPassword("Enter Bearer Token: ")
		if err != nil {
			return nil, err
		}
		return &auth.Profile{
			Method: auth.MethodBearer,
			Token:  token,
		}, nil
	}
}

func printAuthHelp() {
	fmt.Println("Usage:")
	fmt.Println("  downloader auth list                       List vault and domain cache entries")
	fmt.Println("  downloader auth vault init                 Initialize encrypted credential vault")
	fmt.Println("  downloader auth vault passwd               Change vault master password")
	fmt.Println("  downloader auth profile set <name>         Configure a named credential profile")
	fmt.Println("  downloader auth profile list               List all decrypted profiles")
	fmt.Println("  downloader auth profile remove <name>      Remove a profile from the vault")
	fmt.Println("  downloader auth domain set <domain> <prof> Set default profile for a domain")
}

func maskSecret(s string) string {
	if strings.HasPrefix(s, "env:") || strings.HasPrefix(s, "$") {
		return s
	}
	if len(s) <= 6 {
		return "******"
	}
	return s[:3] + "..." + s[len(s)-3:]
}

// handleResume resumes a previously interrupted or paused download session.
func handleResume(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: downloader resume <session-name> [-workers N] [-limit X] [-password-file path]")
		return
	}
	name := args[0]
	fs := flag.NewFlagSet("resume", flag.ExitOnError)
	workers := fs.Int("workers", 3, "Concurrent worker pool size")
	limitStr := fs.String("limit", "", "Bandwidth limit (e.g. 5MB/s, 500KB/s)")
	passFile := fs.String("password-file", "", "Path to file containing vault master password")
	passStdin := fs.Bool("password-stdin", false, "Read vault master password from standard input")
	_ = fs.Parse(args[1:])

	st, err := store.DefaultStore()
	if err != nil {
		printError("Error: %v\n", err)
		os.Exit(1)
	}

	sess, err := st.LoadSession(name)
	if err != nil {
		printError("Error loading session: %v\n", err)
		os.Exit(1)
	}

	masterPW := resolvePasswordFlags(*passFile, *passStdin)

	cfg := engine.Config{
		SessionName:  sess.Name,
		OutputDir:    sess.OutputDir,
		WorkerCount:  *workers,
		RateLimitBps: parseRateLimit(*limitStr),
		MasterPass:   masterPW,
	}

	authMgr, _ := auth.DefaultManager()
	vault, _ := auth.DefaultVault()

	eng := engine.NewEngine(cfg, st, authMgr, vault)
	if err := eng.Run(context.Background(), sess); err != nil {
		printError("Run error: %v\n", err)
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
	passFile := fs.String("password-file", "", "Path to file containing vault master password")
	passStdin := fs.Bool("password-stdin", false, "Read vault master password from standard input")

	_ = fs.Parse(os.Args[1:])

	if *file == "" {
		fmt.Println("Usage: downloader -file urls.txt [-name jobname] [-workers 3] [-limit 5MB/s] [-dir ./downloads] [-password-file key.txt]")
		fs.PrintDefaults()
		return
	}

	sessionName := *name
	if sessionName == "" {
		sessionName = fmt.Sprintf("job_%d", time.Now().Unix())
	}

	st, err := store.DefaultStore()
	if err != nil {
		printError("Error initializing storage: %v\n", err)
		os.Exit(1)
	}

	jobs, err := parseURLFile(*file, *sha256Expected)
	if err != nil {
		printError("Error reading URL file: %v\n", err)
		os.Exit(1)
	}

	sess := &model.Session{
		Name:      sessionName,
		OutputDir: *dir,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Jobs:      jobs,
	}

	masterPW := resolvePasswordFlags(*passFile, *passStdin)

	cfg := engine.Config{
		SessionName:  sessionName,
		OutputDir:    *dir,
		WorkerCount:  *workers,
		RateLimitBps: parseRateLimit(*limitStr),
		MasterPass:   masterPW,
	}

	authMgr, _ := auth.DefaultManager()
	vault, _ := auth.DefaultVault()

	eng := engine.NewEngine(cfg, st, authMgr, vault)
	if err := eng.Run(context.Background(), sess); err != nil {
		printError("Download error: %v\n", err)
	}
}

// resolvePasswordFlags retrieves the master password from a file or stdin if specified.
func resolvePasswordFlags(passFile string, passStdin bool) string {
	if passFile != "" {
		p, err := auth.ReadPasswordFromFile(passFile)
		if err != nil {
			printError("Warning: failed reading password file: %v\n", err)
		} else {
			return p
		}
	}
	if passStdin {
		p, err := auth.ReadPasswordFromStdin()
		if err != nil {
			printError("Warning: failed reading password from stdin: %v\n", err)
		} else {
			return p
		}
	}
	return ""
}

// parseURLFile parses URLs, auth profiles, and SHA-256 hashes from a newline-delimited text file.
func parseURLFile(filePath, singleSHA string) ([]*model.FileJob, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var jobs []*model.FileJob
	scanner := bufio.NewScanner(f)
	idx := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		u, expectedHash, authProfile := parseURLLine(line, singleSHA)
		if u == "" {
			continue
		}

		hash := sha256.Sum256([]byte(u))
		jobID := hex.EncodeToString(hash[:8])

		job := &model.FileJob{
			ID:             fmt.Sprintf("%s_%d", jobID, idx),
			URL:            u,
			TotalSize:      -1,
			Status:         model.StatusPending,
			StreamState:    model.StateIdle,
			ExpectedSHA256: expectedHash,
			AuthProfile:    authProfile,
		}
		jobs = append(jobs, job)
		idx++
	}

	return jobs, scanner.Err()
}

// parseURLLine extracts the target URL, optional expected SHA-256 checksum, and optional auth profile.
// Supported syntax:
//
//	<url> auth=<profile> sha256=<hash>
//	<url> <sha256>
func parseURLLine(line, defaultSHA string) (rawURL, sha256Hash, authProfile string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "", "", ""
	}

	rawURL = parts[0]
	sha256Hash = defaultSHA

	for _, token := range parts[1:] {
		lowerToken := strings.ToLower(token)
		if strings.HasPrefix(lowerToken, "auth=") {
			authProfile = strings.TrimPrefix(token, "auth=")
			authProfile = strings.TrimPrefix(authProfile, "AUTH=")
		} else if strings.HasPrefix(lowerToken, "sha256=") {
			sha256Hash = strings.TrimPrefix(token, "sha256=")
			sha256Hash = strings.TrimPrefix(sha256Hash, "SHA256=")
		} else if len(token) == 64 && isHexString(token) && sha256Hash == "" {
			// Bare 64-character hex string treated as SHA-256
			sha256Hash = token
		}
	}

	return rawURL, sha256Hash, authProfile
}

// isHexString checks if the string consists exclusively of hexadecimal characters.
func isHexString(s string) bool {
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
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
