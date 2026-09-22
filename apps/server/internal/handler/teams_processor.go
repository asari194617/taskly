package handler

import (
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/jobs"
	"gorm.io/gorm"
)

type teamsJob struct {
	Title          string
	Description    *string
	AddedUserID    uint
	SubjectID      uint
	Deadline       time.Time
	StatusCallback *string
}

type teamsProcessor struct {
	db        *gorm.DB
	stop      chan struct{}
	done      chan struct{}
	started   chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
	workers   []*jobs.Worker
}

var integrationJobKinds = []string{"teams_inbound", "teams_callback", "task_discord", "task_email"}

func integrationWorkersPerKind() int {
	n, err := strconv.Atoi(os.Getenv("INTEGRATION_JOB_WORKERS_PER_KIND"))
	if err != nil || n < 1 || n > 8 {
		return 1
	}
	return n
}

func newTeamsProcessor(db *gorm.DB) *teamsProcessor {
	return &teamsProcessor{
		db:      db,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		started: make(chan struct{}),
	}
}

func (p *teamsProcessor) Start() {
	p.startOnce.Do(func() {
		policy := jobs.CallbackPolicy{
			Validate: func(url string) error {
				return validateTeamsCallbackURL(url, os.Getenv("TEAMS_CALLBACK_ALLOWED_ORIGINS"))
			},
			Client: newTeamsCallbackClient(net.DefaultResolver.LookupIPAddr),
		}
		for _, kind := range integrationJobKinds {
			for i := 0; i < integrationWorkersPerKind(); i++ {
				worker := jobs.NewKindWorker(p.db, kind, func(job jobs.Job) error { return jobs.Deliver(p.db, job, policy) })
				p.workers = append(p.workers, worker)
				worker.Start()
			}
		}
		// Preserve failure reporting for unsupported kinds left by older or
		// newer deployments instead of leaving those rows pending forever.
		fallback := jobs.NewWorker(p.db, func(job jobs.Job) error { return jobs.Deliver(p.db, job, policy) })
		fallback.ExcludeKinds = integrationJobKinds
		p.workers = append(p.workers, fallback)
		fallback.Start()
		close(p.started)
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				if err := jobs.ObserveBacklog(p.db, integrationJobKinds); err != nil {
					log.Printf("integration jobs backlog: %v", err)
				}
				select {
				case <-p.stop:
					return
				case <-ticker.C:
				}
			}
		}()
		go func() {
			<-p.stop
			var group sync.WaitGroup
			for _, worker := range p.workers {
				group.Add(1)
				go func(w *jobs.Worker) { defer group.Done(); w.Stop() }(worker)
			}
			group.Wait()
			close(p.done)
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
	_, err := jobs.Enqueue(p.db, "teams_inbound", jobs.TeamsInbound{
		Title: job.Title, Description: job.Description, AddedUserID: job.AddedUserID,
		SubjectID: job.SubjectID, Deadline: job.Deadline, StatusCallback: job.StatusCallback,
	}, nil)
	return err == nil
}
