// Package engine orchestrates the download lifecycle across worker routines,
// coordinating probing, scheduling, transfer execution, watchdog monitoring, and persistence.
package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"downloader/internal/auth"
	"downloader/internal/dashboard"
	"downloader/internal/limiter"
	"downloader/internal/model"
	"downloader/internal/prober"
	"downloader/internal/queue"
	"downloader/internal/store"
	"downloader/internal/transfer"
	"downloader/internal/watchdog"
)

// Config specifies runtime parameters for an Engine instance.
type Config struct {
	SessionName  string
	OutputDir    string
	WorkerCount  int
	RateLimitBps int64
	MaxRetries   int
	ProbeTimeout time.Duration
	MasterPass   string
}

// Engine coordinates workers, rate limiting, persistence, and UI rendering for a download session.
type Engine struct {
	cfg        Config
	store      *store.Store
	downloader *transfer.Downloader
	session    *model.Session
	prober     *prober.Prober
	authMgr    *auth.Manager
	vault      *auth.Vault
}

// NewEngine initializes a new Engine instance with the given configuration, store, auth manager, and encrypted vault.
func NewEngine(cfg Config, st *store.Store, authMgr *auth.Manager, vault *auth.Vault) *Engine {
	if cfg.WorkerCount <= 0 {
		cfg.WorkerCount = 3
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = 5
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "."
	}
	if authMgr == nil {
		var err error
		authMgr, err = auth.DefaultManager()
		if err != nil {
			authMgr = auth.NewManager(nil, auth.NewNonInteractivePrompter())
		}
	}
	if vault == nil {
		vault, _ = auth.DefaultVault()
	}

	rl := limiter.NewRateLimiter(cfg.RateLimitBps)

	return &Engine{
		cfg:        cfg,
		store:      st,
		prober:     prober.NewProber(cfg.ProbeTimeout),
		downloader: transfer.NewDownloader(rl),
		authMgr:    authMgr,
		vault:      vault,
	}
}

// Run executes the complete session lifecycle: probing URLs, sorting tasks, spawning workers, and tracking progress.
func (e *Engine) Run(ctx context.Context, session *model.Session) error {
	e.session = session
	_ = os.MkdirAll(e.cfg.OutputDir, 0755)

	// Context with interrupt handling
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal. Gracefully shutting down...")
		cancel()
	}()

	// 1. Check if vault unlock is required
	needsVault := false
	for _, job := range session.Jobs {
		if job.AuthProfile != "" {
			needsVault = true
			break
		}
	}
	if !needsVault && e.vault != nil && e.vault.Exists() {
		// Vault exists; unlock it so domain defaults or stored profiles can be used
		needsVault = true
	}

	if needsVault && e.vault != nil && e.vault.Exists() && !e.vault.IsUnlocked() {
		if e.cfg.MasterPass != "" {
			if err := e.vault.Unlock(e.cfg.MasterPass); err != nil {
				return fmt.Errorf("failed unlocking vault with provided password: %w", err)
			}
		} else {
			pass, err := auth.PromptPassword("[Vault] Enter Master Password to unlock credentials: ")
			if err != nil {
				return fmt.Errorf("failed prompting for master password: %w", err)
			}
			if err := e.vault.Unlock(pass); err != nil {
				return fmt.Errorf("failed unlocking vault: %w", err)
			}
			fmt.Println("[Vault] Vault unlocked successfully.")
		}
	}

	// 2. Resolve Auth Strategies & Probing Phase
	fmt.Printf("Probing %d URLs for size and resumability...\n", len(session.Jobs))
	for _, job := range session.Jobs {
		if job.Status == model.StatusCompleted {
			continue
		}
		job.Status = model.StatusProbing

		// Resolve domain or profile auth strategy
		strat, authErr := e.resolveAuthStrategy(job)
		if authErr != nil {
			job.LastError = fmt.Sprintf("auth error: %v", authErr)
			job.Status = model.StatusFailed
			continue
		}

		if err := e.prober.PopulateJobWithAuth(ctx, job, strat); err != nil {
			job.LastError = err.Error()
		}
		job.Status = model.StatusPending
	}
	_ = e.store.SaveSession(session)

	// 3. Initialize Queue and Sort (Smallest to largest, unknown last)
	var activeJobs []*model.FileJob
	for _, j := range session.Jobs {
		if j.Status != model.StatusCompleted {
			activeJobs = append(activeJobs, j)
		}
	}

	if len(activeJobs) == 0 {
		fmt.Println("All files are already downloaded.")
		printSummary(session)
		return nil
	}

	remainingJobs := int64(len(activeJobs))

	q := queue.NewQueue(activeJobs)
	go func() {
		<-ctx.Done()
		q.Close()
	}()

	wd := watchdog.NewWatchdog(q)
	dash := dashboard.NewDashboard(session.Jobs, e.cfg.WorkerCount)
	dash.Start(200 * time.Millisecond)
	defer dash.Stop()

	// 4. Watchdog Loop
	wdTicker := time.NewTicker(1 * time.Second)
	defer wdTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-wdTicker.C:
				demotedJobs := wd.Tick()
				for _, demoted := range demotedJobs {
					q.Demote(demoted)
				}
			}
		}
	}()

	// Periodic session auto-save
	saveTicker := time.NewTicker(5 * time.Second)
	defer saveTicker.Stop()
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-saveTicker.C:
				_ = e.store.SaveSession(session)
			}
		}
	}()

	// 5. Worker Pool
	var wg sync.WaitGroup
	slotJobs := make([]*model.FileJob, e.cfg.WorkerCount)
	var slotMu sync.Mutex

	for wId := 0; wId < e.cfg.WorkerCount; wId++ {
		wg.Add(1)
		workerIndex := wId
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				job, ok := q.Pop()
				if !ok {
					slotMu.Lock()
					slotJobs[workerIndex] = nil
					dash.SetActiveJobs(slotJobs)
					slotMu.Unlock()
					return
				}

				slotMu.Lock()
				slotJobs[workerIndex] = job
				dash.SetActiveJobs(slotJobs)
				slotMu.Unlock()

				e.executeJob(ctx, job, wd)

				slotMu.Lock()
				slotJobs[workerIndex] = nil
				dash.SetActiveJobs(slotJobs)
				slotMu.Unlock()

				if job.Status == model.StatusCompleted {
					if atomic.AddInt64(&remainingJobs, -1) <= 0 {
						q.Close()
					}
				} else if job.Status == model.StatusFailed {
					// Distinguish auth error vs transient error
					isAuthFatal := false
					if auth.IsUnauthorized(errorsNew(job.LastError)) || auth.IsForbidden(errorsNew(job.LastError)) {
						isAuthFatal = true
					}

					if !isAuthFatal && job.RetryCount < e.cfg.MaxRetries && ctx.Err() == nil {
						job.RetryCount++
						backoff := time.Duration(1<<job.RetryCount) * time.Second
						time.Sleep(backoff)
						job.Status = model.StatusPending
						q.Push(job)
					} else {
						// Permanently failed
						if atomic.AddInt64(&remainingJobs, -1) <= 0 {
							q.Close()
						}
					}
				}
			}
		}()
	}

	wg.Wait()
	q.Close()

	// Final session save
	_ = e.store.SaveSession(session)
	dash.Render()

	// Summary output
	printSummary(session)
	return nil
}

// resolveAuthStrategy determines the Strategy for a job checking explicit profile, vault domain default, and domain cache.
func (e *Engine) resolveAuthStrategy(job *model.FileJob) (auth.Strategy, error) {
	// 1. Explicit profile specified in URL list (e.g. auth=my-profile)
	if job.AuthProfile != "" {
		if e.vault != nil && e.vault.IsUnlocked() {
			return e.vault.ResolveStrategy(job.AuthProfile, job.URL)
		}
		return nil, fmt.Errorf("job requires auth profile %q, but encrypted vault is locked or not found", job.AuthProfile)
	}

	// 2. Vault domain default mapping
	if e.vault != nil && e.vault.IsUnlocked() {
		strat, err := e.vault.ResolveStrategy("", job.URL)
		if err == nil && strat.Type() != auth.MethodNone {
			return strat, nil
		}
	}

	// 3. Fallback to domain cache (~/.downloader/auth) or prompter
	if e.authMgr != nil {
		return e.authMgr.ResolveURL(job.URL)
	}

	return auth.NewAnonymousStrategy(), nil
}

func errorsNew(s string) error {
	if s == "" {
		return nil
	}
	return fmt.Errorf("%s", s)
}

// executeJob processes an individual download job under watchdog supervision.
func (e *Engine) executeJob(ctx context.Context, job *model.FileJob, wd *watchdog.Watchdog) {
	jobCtx, jobCancel := context.WithCancel(ctx)
	defer jobCancel()

	job.Status = model.StatusInProgress
	wd.RegisterStream(job, jobCancel)
	defer wd.UnregisterStream(job.ID)

	strat, authErr := e.resolveAuthStrategy(job)
	if authErr != nil {
		job.Status = model.StatusFailed
		job.LastError = fmt.Sprintf("auth error: %v", authErr)
		return
	}

	err := e.downloader.Download(jobCtx, job, e.cfg.OutputDir, strat)
	if err != nil {
		if ctx.Err() != nil {
			job.Status = model.StatusPaused
			return
		}
		if job.Status == model.StatusDemoted {
			return
		}
		job.Status = model.StatusFailed
		job.LastError = err.Error()

		// If rate-limited with a Retry-After header, sleep for Retry-After duration
		if authErr, ok := auth.AsAuthError(err); ok && authErr.Kind == auth.KindRateLimited && authErr.RetryAfter > 0 {
			select {
			case <-ctx.Done():
			case <-time.After(authErr.RetryAfter):
			}
		}
	} else {
		job.Status = model.StatusCompleted
	}
}

// printSummary outputs a formatted terminal report detailing final session results.
func printSummary(session *model.Session) {
	fmt.Println("\n================ Download Summary ================")
	var completed, failed, paused int
	var totalBytes int64

	for _, j := range session.Jobs {
		switch j.Status {
		case model.StatusCompleted:
			completed++
			totalBytes += j.DownloadedBytes
			fmt.Printf(" [SUCCESS] %-25s (%s)\n", j.ResolvedFilename, model.FormatBytes(j.DownloadedBytes))
		case model.StatusFailed:
			failed++
			fmt.Printf(" [FAILED]  %-25s (Error: %s)\n", j.ResolvedFilename, j.LastError)
		default:
			paused++
			fmt.Printf(" [PAUSED]  %-25s (%s downloaded)\n", j.ResolvedFilename, model.FormatBytes(j.DownloadedBytes))
		}
	}
	fmt.Printf("--------------------------------------------------\n")
	fmt.Printf("Total: %d | Completed: %d | Failed: %d | Paused: %d | Data: %s\n",
		len(session.Jobs), completed, failed, paused, model.FormatBytes(totalBytes))
	fmt.Println("==================================================")
}
