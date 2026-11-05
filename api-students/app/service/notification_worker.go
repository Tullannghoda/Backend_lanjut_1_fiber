package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/hibiken/asynq"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

// NotificationWorker mengerjakan task "notification:send".
//
// Bagi worker, Asynq adalah framework-nya — sama seperti Fiber bagi API.
// Karena itu method di sini menerima *asynq.Task, seperti method service
// lain menerima *fiber.Ctx.
type NotificationWorker struct {
	users         repository.UserRepository
	courses       repository.CourseRepository
	notifications repository.NotificationRepository
	tx            *repository.TxManager
	mailer        helper.Mailer
	logger        *slog.Logger
}

func NewNotificationWorker(
	users repository.UserRepository,
	courses repository.CourseRepository,
	notifications repository.NotificationRepository,
	tx *repository.TxManager,
	mailer helper.Mailer,
	logger *slog.Logger,
) *NotificationWorker {
	return &NotificationWorker{
		users:         users,
		courses:       courses,
		notifications: notifications,
		tx:            tx,
		mailer:        mailer,
		logger:        logger,
	}
}

// HandleSend mengembalikan error untuk meminta Asynq melakukan retry nanti,
// atau error yang membungkus asynq.SkipRetry untuk menyatakan "jangan di-retry,
// task ini tidak mungkin berhasil".
//
// Bug 3 fixed: parameter bertipe *asynq.Task (pointer)
func (w *NotificationWorker) HandleSend(ctx context.Context, t *asynq.Task) error {
	var job model.NotificationJob
	if err := json.Unmarshal(t.Payload(), &job); err != nil {
		return fmt.Errorf("%w: payload task rusak: %v", asynq.SkipRetry, err)
	}

	// Tugas Mandiri D.2: Penanganan event course.quota_changed untuk seluruh peserta
	if job.EventType == model.EventCourseQuotaChanged {
		return w.handleQuotaChanged(ctx, job)
	}

	var event model.EnrollmentEvent
	if err := json.Unmarshal(job.Payload, &event); err != nil {
		return fmt.Errorf("%w: payload event %d rusak: %v", asynq.SkipRetry, job.OutboxID, err)
	}

	user, err := w.users.FindByID(ctx, event.UserID)
	if errors.Is(err, repository.ErrNotFound) {
		// Akunnya sudah dihapus. Melakukan retry tidak akan memunculkannya.
		return fmt.Errorf("%w: user %d tidak ditemukan", asynq.SkipRetry, event.UserID)
	}
	if err != nil {
		return err // database sedang bermasalah: layak di-retry
	}

	msg, err := BuildMessage(job.EventType, event, user)
	if err != nil {
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}

	alreadySent := false
	err = w.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
		// Claim dan pengiriman berada di dalam SATU transaction:
		//   - pengiriman gagal  -> claim ikut dibatalkan -> attempt berikutnya masih dapat mengirim;
		//   - pengiriman sukses -> claim di-commit -> attempt berikutnya melihat claim ini dan tidak mengirim lagi.
		// Bug 8 fixed: panggil w.notifications.WithTx(tx) agar claim berada di transaction yang sama
		claimed, err := w.notifications.WithTx(tx).Claim(ctx, model.Notification{
			OutboxID: job.OutboxID, UserID: user.ID, Channel: "email",
			Subject: msg.Subject, Body: msg.Body,
		})
		if err != nil {
			return err
		}
		if !claimed {
			alreadySent = true
			return nil
		}
		return w.mailer.Send(ctx, msg)
	})

	if errors.Is(err, helper.ErrPermanentDelivery) {
		// Bug 9 fixed: %w untuk asynq.SkipRetry agar Asynq tidak mengulang task yang gagal permanen
		return fmt.Errorf("%w: %v", asynq.SkipRetry, err)
	}
	if err != nil {
		return err
	}

	retry, _ := asynq.GetRetryCount(ctx)
	w.logger.Info("notification_processed",
		slog.Int64("outbox_id", job.OutboxID),
		slog.String("event", job.EventType),
		slog.Int("retry", retry),
		slog.Bool("already_sent", alreadySent))
	return nil
}

// handleQuotaChanged mengirim notifikasi ke seluruh peserta kelas secara idempoten per-user
func (w *NotificationWorker) handleQuotaChanged(ctx context.Context, job model.NotificationJob) error {
	var event model.CourseQuotaChangedEvent
	if err := json.Unmarshal(job.Payload, &event); err != nil {
		return fmt.Errorf("%w: payload quota event %d rusak: %v", asynq.SkipRetry, job.OutboxID, err)
	}

	participants, err := w.courses.FindEnrolledUsers(ctx, event.CourseID)
	if err != nil {
		return fmt.Errorf("mengambil peserta kelas %d: %w", event.CourseID, err)
	}

	for _, user := range participants {
		msg := BuildQuotaChangedMessage(event, user)
		alreadySent := false

		// Setiap email memiliki transaksi sendiri: bila email ke-7 gagal,
		// claim email ke-7 batal, namun email 1-6 tetap ter-commit dan tidak diulang pada retry
		err := w.tx.WithTxOnce(ctx, func(tx repository.DBTX) error {
			claimed, err := w.notifications.WithTx(tx).Claim(ctx, model.Notification{
				OutboxID: job.OutboxID,
				UserID:   user.ID,
				Channel:  "email",
				Subject:  msg.Subject,
				Body:     msg.Body,
			})
			if err != nil {
				return err
			}
			if !claimed {
				alreadySent = true
				return nil
			}
			return w.mailer.Send(ctx, msg)
		})

		if errors.Is(err, helper.ErrPermanentDelivery) {
			w.logger.Warn("permanent_email_skipped", slog.String("email", user.Email))
			continue
		}
		if err != nil {
			return err // hentikan loop dan minta Asynq retry
		}

		retry, _ := asynq.GetRetryCount(ctx)
		w.logger.Info("notification_processed",
			slog.Int64("outbox_id", job.OutboxID),
			slog.Int("user_id", user.ID),
			slog.String("event", job.EventType),
			slog.Int("retry", retry),
			slog.Bool("already_sent", alreadySent))
	}

	return nil
}
