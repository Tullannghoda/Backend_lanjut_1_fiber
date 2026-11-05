package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/hibiken/asynq"

	"api-students/app/model"
	"api-students/config"
	"api-students/database"
)

func main() {
	outboxID := flag.Int64("outbox", 0, "ID outbox yang akan dikirim ulang")
	copies := flag.Int("copies", 1, "Jumlah salinan task yang akan di-enqueue")
	flag.Parse()

	if *outboxID <= 0 {
		fmt.Println("Gunakan: go run ./tools/redeliver -outbox <id> [-copies <n>]")
		os.Exit(1)
	}

	config.LoadEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := database.NewPool(ctx)
	if err != nil {
		fmt.Printf("Gagal terhubung ke database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	var event model.OutboxEvent
	err = pool.QueryRow(ctx,
		`SELECT id, event_type, payload, created_at FROM outbox WHERE id = $1`, *outboxID).
		Scan(&event.ID, &event.EventType, &event.Payload, &event.CreatedAt)
	if err != nil {
		fmt.Printf("Gagal membaca outbox %d: %v\n", *outboxID, err)
		os.Exit(1)
	}

	job := model.NotificationJob{
		OutboxID:  event.ID,
		EventType: event.EventType,
		Payload:   event.Payload,
	}

	payload, err := json.Marshal(job)
	if err != nil {
		fmt.Printf("Gagal menyusun payload JSON: %v\n", err)
		os.Exit(1)
	}

	client := asynq.NewClient(config.RedisOpt())
	defer client.Close()

	for i := 1; i <= *copies; i++ {
		task := asynq.NewTask(model.TaskSendNotification, payload,
			asynq.TaskID(fmt.Sprintf("redeliver-%d-%d-%d", event.ID, i, time.Now().UnixNano())),
			asynq.Queue(model.QueueNotifications),
			asynq.MaxRetry(5),
			asynq.Timeout(30*time.Second),
		)

		info, err := client.EnqueueContext(ctx, task)
		if err != nil {
			fmt.Printf("Gagal enqueue salinan ke-%d: %v\n", i, err)
			os.Exit(1)
		}
		fmt.Printf("Salinan %d/%d di-enqueue: task_id=%s queue=%s\n", i, *copies, info.ID, info.Queue)
	}

	fmt.Printf("Berhasil mengirim %d salinan task outbox %d ke queue %s\n",
		*copies, event.ID, model.QueueNotifications)
}
