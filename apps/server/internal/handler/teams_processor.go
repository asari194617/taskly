package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
)

type teamsJob struct {
	Title          string
	Description    *string
	AddedUserID    uint
	SubjectID      uint
	StatusCallback *string
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type teamsProcessor struct {
	db        *gorm.DB
	queue     chan teamsJob
	stop      chan struct{}
	done      chan struct{}
	started   chan struct{}
	client    httpDoer
	sleep     func(time.Duration)
	secret    func() string
	startOnce sync.Once
	stopOnce  sync.Once
}

func newTeamsProcessor(db *gorm.DB) *teamsProcessor {
	return &teamsProcessor{
		db:      db,
		queue:   make(chan teamsJob, 100),
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		started: make(chan struct{}),
		client:  newTeamsCallbackClient(net.DefaultResolver.LookupIPAddr),
		sleep:   time.Sleep,
		secret:  func() string { return os.Getenv("TEAMS_INTEGRATION_SECRET") },
	}
}

func (p *teamsProcessor) Start() {
	p.startOnce.Do(func() {
		close(p.started)
		go func() {
			defer close(p.done)
			for {
				select {
				case job := <-p.queue:
					p.process(job)
				case <-p.stop:
					// Finish work already accepted before shutdown.
					for {
						select {
						case job := <-p.queue:
							p.process(job)
						default:
							return
						}
					}
				}
			}
		}()
	})
}

func (p *teamsProcessor) Stop() {
	p.stopOnce.Do(func() { close(p.stop) })
	select {
	case <-p.started:
	default:
		return
	}
	select {
	case <-p.done:
	case <-time.After(5 * time.Second):
	}
}

func (p *teamsProcessor) Enqueue(job teamsJob) bool {
	select {
	case <-p.stop:
		return false
	default:
	}
	select {
	case p.queue <- job:
		return true
	default:
		return false
	}
}

func (p *teamsProcessor) process(job teamsJob) {
	record := domain.Task{
		AddedUserID: job.AddedUserID,
		Title:       job.Title,
		SubjectID:   job.SubjectID,
		Description: job.Description,
	}
	createErr := p.db.Create(&record).Error
	succeeded := createErr == nil
	status := "created"
	if !succeeded {
		status = "create_failed"
	}
	_ = p.db.Table("tasks_teams_logs").Create(map[string]any{
		"task_id":      record.ID,
		"is_success":   succeeded,
		"callback_url": job.StatusCallback,
		"attempts":     0,
		"last_status":  status,
	}).Error

	if job.StatusCallback != nil && *job.StatusCallback != "" {
		p.sendCallback(record.ID, succeeded, *job.StatusCallback)
	}
}

func (p *teamsProcessor) sendCallback(taskID uint, succeeded bool, callbackURL string) {
	if err := validateTeamsCallbackURL(callbackURL, os.Getenv("TEAMS_CALLBACK_ALLOWED_ORIGINS")); err != nil {
		_ = p.db.Table("tasks_teams_logs").Where("task_id = ?", taskID).Updates(map[string]any{
			"last_status": "callback_rejected",
			"last_error":  err.Error(),
		}).Error
		return
	}
	payload := fmt.Sprintf(`{"is_success":%t,"task_id":%d}`, succeeded, taskID)
	signature := ""
	if secret := p.secret(); secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(payload))
		signature = fmt.Sprintf("sha256=%x", mac.Sum(nil))
	}

	for attempt := 1; attempt <= 3; attempt++ {
		req, err := http.NewRequest(http.MethodPost, callbackURL, strings.NewReader(payload))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			if signature != "" {
				req.Header.Set("X-Signature", signature)
			}
			var resp *http.Response
			resp, err = p.client.Do(req)
			if err == nil {
				body := []byte{}
				if resp.Body != nil {
					body, _ = io.ReadAll(io.LimitReader(resp.Body, 64*1024))
					_ = resp.Body.Close()
				}
				_ = p.db.Table("tasks_teams_logs").Where("task_id = ?", taskID).Updates(map[string]any{
					"attempts":             attempt,
					"last_status":          "callback_sent",
					"last_response_status": resp.StatusCode,
					"last_response_body":   string(body),
				}).Error
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					return
				}
			}
		}
		if err != nil {
			_ = p.db.Table("tasks_teams_logs").Where("task_id = ?", taskID).Updates(map[string]any{
				"attempts":    attempt,
				"last_status": "callback_error",
				"last_error":  err.Error(),
			}).Error
		}
		if attempt < 3 {
			p.sleep(time.Duration(attempt) * time.Second)
		}
	}
}
