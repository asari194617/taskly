package jobs

import (
	"bufio"
	"encoding/json"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/asari194617/taskly/apps/server/internal/domain"
	"gorm.io/gorm"
)

func pendingTeamsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("TEAMS_NOTIFY_EMAIL", "teams@example.test")
	t.Setenv("DISCORD_WEBHOOK_URL", "")
	db := testDB(t)
	createTasksTable(t, db)
	if err := db.Exec(`CREATE TABLE tasks_teams_logs (
		id integer primary key autoincrement, task_id integer not null,
		teams_message_id text, teams_conversation_id text,
		inbound_job_id integer unique, is_success integer not null default 0,
		callback_url text, attempts integer not null default 0,
		last_status text, last_error text, last_response_status integer,
		last_response_body text
	)`).Error; err != nil {
		t.Fatal(err)
	}
	return db
}

func createPendingTeamsTask(t *testing.T, db *gorm.DB) domain.Task {
	t.Helper()
	task, err := CreateTask(db, 1, "before", 1, nil, "OTHER", false, false,
		domain.Task{Deadline: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), DeadlineSince: "NONE", DeadlineUntil: "NONE"}, "actor@example.test")
	if err != nil {
		t.Fatal(err)
	}
	created := pendingEmailJob(t, db, "created")
	// The add email has been sent, but Power Automate has not reported its ID.
	if err := db.Model(&Job{}).Where("id = ?", created.ID).Update("status", "succeeded").Error; err != nil {
		t.Fatal(err)
	}
	return task
}

func emailJobsForAction(t *testing.T, db *gorm.DB, action string) []Job {
	t.Helper()
	var rows []Job
	if err := db.Where("kind = ?", "task_email").Order("id").Find(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var matches []Job
	for _, row := range rows {
		var notice TaskNotice
		if err := json.Unmarshal([]byte(row.Payload), &notice); err != nil {
			t.Fatal(err)
		}
		if notice.Action == action {
			matches = append(matches, row)
		}
	}
	return matches
}

func pendingEmailJob(t *testing.T, db *gorm.DB, action string) Job {
	t.Helper()
	matches := emailJobsForAction(t, db, action)
	if len(matches) != 1 {
		t.Fatalf("%s email jobs = %d, want 1", action, len(matches))
	}
	return matches[0]
}

func setJobDue(t *testing.T, db *gorm.DB, jobID uint64) {
	t.Helper()
	if err := db.Model(&Job{}).Where("id = ?", jobID).Update("available_at", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
}

func insertTeamsMessageID(t *testing.T, db *gorm.DB, taskID uint, messageID string) {
	t.Helper()
	if err := db.Table("tasks_teams_logs").Create(map[string]any{
		"task_id": taskID, "teams_message_id": messageID,
		"is_success": true, "last_status": "callback_received",
	}).Error; err != nil {
		t.Fatal(err)
	}
}

// startTeamsSMTP captures a single email without depending on an external SMTP service.
func startTeamsSMTP(t *testing.T) <-chan string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("SMTP_HOST", host)
	t.Setenv("SMTP_PORT", port)
	t.Setenv("SMTP_TIMEOUT", "2")
	t.Setenv("SMTP_TLS", "")
	t.Setenv("SMTP_PASSWORD", "")
	mail := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		reader := bufio.NewReader(conn)
		_, _ = conn.Write([]byte("220 test SMTP\r\n"))
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO "), strings.HasPrefix(line, "HELO "):
				_, _ = conn.Write([]byte("250 test SMTP\r\n"))
			case strings.HasPrefix(line, "MAIL FROM:"), strings.HasPrefix(line, "RCPT TO:"):
				_, _ = conn.Write([]byte("250 OK\r\n"))
			case strings.HasPrefix(line, "DATA"):
				_, _ = conn.Write([]byte("354 send message\r\n"))
				var body strings.Builder
				for {
					part, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if part == ".\r\n" {
						break
					}
					body.WriteString(part)
				}
				mail <- body.String()
				_, _ = conn.Write([]byte("250 queued\r\n"))
			case strings.HasPrefix(line, "QUIT"):
				_, _ = conn.Write([]byte("221 bye\r\n"))
				return
			default:
				_, _ = conn.Write([]byte("500 unsupported\r\n"))
				return
			}
		}
	}()
	return mail
}

func assertDeferredEmail(t *testing.T, db *gorm.DB, jobID uint64) {
	t.Helper()
	worker := NewKindWorker(db, "task_email", func(job Job) error { return Deliver(db, job, CallbackPolicy{}) })
	setJobDue(t, db, jobID)
	processed, err := worker.RunOne()
	if err != nil || !processed {
		t.Fatalf("deferred email: processed=%t err=%v", processed, err)
	}
	var stored Job
	if err := db.First(&stored, jobID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "pending" || stored.Attempts != 0 {
		t.Fatalf("deferred email status=%q attempts=%d, want pending/0", stored.Status, stored.Attempts)
	}
}

func deliverPendingEmail(t *testing.T, db *gorm.DB, jobID uint64) string {
	t.Helper()
	mail := startTeamsSMTP(t)
	setJobDue(t, db, jobID)
	worker := NewKindWorker(db, "task_email", func(job Job) error { return Deliver(db, job, CallbackPolicy{}) })
	processed, err := worker.RunOne()
	if err != nil || !processed {
		t.Fatalf("reply email: processed=%t err=%v", processed, err)
	}
	var stored Job
	if err := db.First(&stored, jobID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "succeeded" {
		t.Fatalf("reply email status=%q, want succeeded", stored.Status)
	}
	select {
	case message := <-mail:
		return message
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP did not receive reply")
		return ""
	}
}

func TestTeamsUpdateWaitsForLateMessageIDWithoutExhaustingRetries(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
		rec.Title = "after"
		return nil
	}, "actor@example.test")
	if err != nil {
		t.Fatal(err)
	}
	job := pendingEmailJob(t, db, "updated")
	for attempt := 0; attempt < maxAttempts+1; attempt++ {
		assertDeferredEmail(t, db, job.ID)
	}
	insertTeamsMessageID(t, db, task.ID, "late-message-id")
	message := deliverPendingEmail(t, db, job.ID)
	if !strings.Contains(message, `"message_id":"late-message-id"`) || !strings.Contains(message, "after") {
		t.Fatalf("reply does not target late Teams message or include update: %q", message)
	}
}

func TestTeamsDeleteWaitsForLateMessageID(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	if _, err := DeleteTask(db, task.ID, "actor@example.test"); err != nil {
		t.Fatal(err)
	}
	job := pendingEmailJob(t, db, "deleted")
	assertDeferredEmail(t, db, job.ID)
	insertTeamsMessageID(t, db, task.ID, "deleted-message-id")
	message := deliverPendingEmail(t, db, job.ID)
	if !strings.Contains(message, `"message_id":"deleted-message-id"`) || !strings.Contains(message, "削除") {
		t.Fatalf("reply does not target deleted Teams message: %q", message)
	}
}

func TestTeamsUpdateWithoutOutboundAddDoesNotQueueReply(t *testing.T) {
	for _, withInboundLog := range []bool{false, true} {
		t.Run(strconv.FormatBool(withInboundLog), func(t *testing.T) {
			db := pendingTeamsTestDB(t)
			task := domain.Task{AddedUserID: 1, Title: "before", SubjectID: 1, Type: "OTHER",
				Deadline: time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC), DeadlineSince: "NONE", DeadlineUntil: "NONE"}
			if err := db.Create(&task).Error; err != nil {
				t.Fatal(err)
			}
			if withInboundLog {
				if err := db.Table("tasks_teams_logs").Create(map[string]any{
					"task_id": task.ID, "inbound_job_id": 123,
					"is_success": true, "last_status": "created",
				}).Error; err != nil {
					t.Fatal(err)
				}
			}
			_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
				rec.Title = "after"
				return nil
			}, "actor@example.test")
			if err != nil {
				t.Fatal(err)
			}
			var count int64
			if err := db.Model(&Job{}).Where("kind = ?", "task_email").Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 0 {
				t.Fatalf("email jobs = %d, want 0", count)
			}
		})
	}
}

func TestTeamsUpdateRecognizesExistingAddJobWithoutMessageID(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	var logCount int64
	if err := db.Table("tasks_teams_logs").Where("task_id = ?", task.ID).Count(&logCount).Error; err != nil {
		t.Fatal(err)
	}
	if logCount != 0 {
		t.Fatalf("Teams log rows before callback = %d, want 0", logCount)
	}
	_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
		rec.Title = "after"
		return nil
	}, "actor@example.test")
	if err != nil {
		t.Fatal(err)
	}
	job := pendingEmailJob(t, db, "updated")
	if job.Status != "pending" {
		t.Fatalf("outbound update status = %q", job.Status)
	}
}

func TestTeamsUpdateAfterMessageIDQueuesImmediateReply(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	insertTeamsMessageID(t, db, task.ID, "already-posted")
	_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
		rec.Title = "after"
		return nil
	}, "actor@example.test")
	if err != nil {
		t.Fatal(err)
	}
	job := pendingEmailJob(t, db, "updated")
	var notice TaskNotice
	if err := json.Unmarshal([]byte(job.Payload), &notice); err != nil {
		t.Fatal(err)
	}
	if notice.MessageID != "already-posted" || !strings.Contains(notice.Message, "after") {
		t.Fatalf("callback-before-update notice = %+v", notice)
	}
}

func TestTeamsFailedCallbackDoesNotReleasePendingReply(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	if err := db.Table("tasks_teams_logs").Create(map[string]any{
		"task_id": task.ID, "teams_message_id": "failed-post",
		"is_success": false, "last_status": "callback_received",
	}).Error; err != nil {
		t.Fatal(err)
	}
	_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
		rec.Title = "after"
		return nil
	}, "actor@example.test")
	if err != nil {
		t.Fatal(err)
	}
	job := pendingEmailJob(t, db, "updated")
	assertDeferredEmail(t, db, job.ID)
	insertTeamsMessageID(t, db, task.ID, "successful-post")
	message := deliverPendingEmail(t, db, job.ID)
	if !strings.Contains(message, `"message_id":"successful-post"`) {
		t.Fatalf("reply targeted unsuccessful Teams post: %q", message)
	}
}

func TestTeamsRepliesForOneTaskKeepMutationOrder(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	for _, title := range []string{"first edit", "second edit"} {
		_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
			rec.Title = title
			return nil
		}, "actor@example.test")
		if err != nil {
			t.Fatal(err)
		}
	}
	updates := emailJobsForAction(t, db, "updated")
	if len(updates) != 2 || updates[0].ID >= updates[1].ID {
		t.Fatalf("update email jobs = %+v, want two in mutation order", updates)
	}
	insertTeamsMessageID(t, db, task.ID, "posted-id")
	// Only the second job is due. It must wait for the earlier reply even
	// though the Teams message ID has already arrived.
	if err := db.Model(&Job{}).Where("id = ?", updates[0].ID).
		Update("available_at", time.Now().Add(time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	setJobDue(t, db, updates[1].ID)
	worker := NewKindWorker(db, "task_email", func(job Job) error { return Deliver(db, job, CallbackPolicy{}) })
	if _, err := worker.RunOne(); err != nil {
		t.Fatal(err)
	}
	var second Job
	if err := db.First(&second, updates[1].ID).Error; err != nil {
		t.Fatal(err)
	}
	if second.Status != "pending" || second.Attempts != 0 {
		t.Fatalf("second reply overtook first: status=%q attempts=%d", second.Status, second.Attempts)
	}
	firstEmail := deliverPendingEmail(t, db, updates[0].ID)
	secondEmail := deliverPendingEmail(t, db, updates[1].ID)
	if !strings.Contains(firstEmail, "first edit") || !strings.Contains(secondEmail, "second edit") {
		t.Fatalf("reply order/content wrong: first=%q second=%q", firstEmail, secondEmail)
	}
}

func TestTeamsPendingEmailJobHasUsefulChangeMessage(t *testing.T) {
	db := pendingTeamsTestDB(t)
	task := createPendingTeamsTask(t, db)
	_, _, err := UpdateTask(db, task.ID, func(rec *domain.Task) error {
		rec.Title = "after"
		return nil
	}, "actor@example.test")
	if err != nil {
		t.Fatal(err)
	}
	job := pendingEmailJob(t, db, "updated")
	var notice TaskNotice
	if err := json.Unmarshal([]byte(job.Payload), &notice); err != nil {
		t.Fatal(err)
	}
	if notice.MessageID != "" || !strings.Contains(notice.Message, "before") || !strings.Contains(notice.Message, "after") {
		t.Fatalf("pending update notice = %+v", notice)
	}
	if job.Status != "pending" {
		t.Fatalf("pending update status = %q", job.Status)
	}
}
