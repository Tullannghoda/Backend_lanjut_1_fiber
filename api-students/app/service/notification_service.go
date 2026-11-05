package service

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/hibiken/asynq"

	"api-students/app/model"
	"api-students/app/repository"
	"api-students/helper"
)

// QueueInspector adalah kemampuan membaca keadaan queue.
// *asynq.Inspector memenuhi interface ini.
type QueueInspector interface {
	GetQueueInfo(queue string) (*asynq.QueueInfo, error)
}

// NotificationService melayani endpoint HTTP seputar notifikasi.
type NotificationService struct {
	notifications repository.NotificationRepository
	outbox        repository.OutboxRepository
	inspector     QueueInspector
}

func NewNotificationService(
	notifications repository.NotificationRepository,
	outbox repository.OutboxRepository,
	inspector QueueInspector,
) *NotificationService {
	return &NotificationService{notifications: notifications, outbox: outbox, inspector: inspector}
}

// ---------- GET /notifications/mine ----------

func (s *NotificationService) Mine(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	current, ok := helper.CurrentUser(c)
	if !ok {
		return helper.Unauthorized("belum terautentikasi")
	}

	list, err := s.notifications.FindByUser(ctx, current.UserID, 20)
	if err != nil {
		return helper.Internal(err)
	}
	return helper.Success(c, fiber.StatusOK, "notifikasi berhasil diambil", list)
}

// ---------- GET /admin/queue ----------
// Dijaga middleware dengan permission queue:monitor.

func (s *NotificationService) Monitor(c *fiber.Ctx) error {
	ctx, cancel := helper.RequestContext(c)
	defer cancel()

	pending, published, oldest, err := s.outbox.Stats(ctx)
	if err != nil {
		return helper.Internal(err)
	}
	sent, err := s.notifications.Count(ctx)
	if err != nil {
		return helper.Internal(err)
	}

	stats := model.QueueStats{
		OutboxPending:     pending,
		OutboxPublished:   published,
		OldestPendingAge:  oldest.Round(time.Millisecond).String(),
		NotificationsSent: sent,
	}

	// Redis yang mati tidak boleh membuat endpoint ini gagal seluruhnya.
	// Justru saat Redis mati, admin paling membutuhkan angka outbox_pending
	// untuk melihat berapa notifikasi yang sedang tertahan.
	info, err := s.inspector.GetQueueInfo(model.QueueNotifications)
	if err != nil && (errors.Is(err, asynq.ErrQueueNotFound) || strings.Contains(err.Error(), "does not exist")) {
		stats.Queue = &model.QueueInfo{
			Name:      model.QueueNotifications,
			Pending:   0,
			Active:    0,
			Scheduled: 0,
			Retry:     0,
			Archived:  0,
			Completed: 0,
			Processed: 0,
			Failed:    0,
		}
	} else if err != nil {
		stats.QueueError = fmt.Sprintf("queue tidak dapat dibaca: %v", err)
	} else {
		stats.Queue = &model.QueueInfo{
			Name:      info.Queue,
			Pending:   info.Pending,
			Active:    info.Active,
			Scheduled: info.Scheduled,
			Retry:     info.Retry,
			Archived:  info.Archived,
			Completed: info.Completed,
			Processed: info.Processed,
			Failed:    info.Failed,
		}
	}
	return helper.Success(c, fiber.StatusOK, "keadaan queue berhasil diambil", stats)
}
