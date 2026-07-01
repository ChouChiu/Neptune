package util

import (
	"log/slog"
	"time"
)

const (
	sendInterval    = time.Second
	maxSendQueueCap = 10
)

type sendJob struct {
	run    func() error
	result chan string // "sent" | "failed"
}

// SendQueue is a rate-limited send queue using a buffered channel.
type SendQueue struct {
	ch chan sendJob
}

// NewSendQueue creates a new SendQueue and starts the worker goroutine.
func NewSendQueue() *SendQueue {
	sq := &SendQueue{ch: make(chan sendJob, maxSendQueueCap)}
	go sq.worker()
	return sq
}

// worker reads from the channel and processes jobs with rate limiting.
// The goroutine exits when the channel is closed.
func (sq *SendQueue) worker() {
	for job := range sq.ch {
		err := job.run()
		if err != nil {
			slog.Error("Telegram send error", "err", err)
			job.result <- "failed"
		} else {
			job.result <- "sent"
		}
		time.Sleep(sendInterval)
	}
}

// Enqueue adds a job to the queue. Returns "dropped" if the queue is full.
func (sq *SendQueue) Enqueue(run func() error) string {
	resultCh := make(chan string, 1)
	select {
	case sq.ch <- sendJob{run: run, result: resultCh}:
		return <-resultCh
	default:
		return "dropped"
	}
}
