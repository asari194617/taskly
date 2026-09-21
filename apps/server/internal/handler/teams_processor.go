package handler

import (
	"net"
	"os"
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
	worker    *jobs.Worker
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
		p.worker = jobs.NewWorker(p.db, func(job jobs.Job) error { return jobs.Deliver(p.db, job, policy) })
		p.worker.Start()
		close(p.started)
		go func() { <-p.stop; p.worker.Stop(); close(p.done) }()
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
