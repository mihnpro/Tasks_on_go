package main

import (
	"broker-comparison/internal/broker"
	"broker-comparison/internal/metrics"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

var (
	brokerType  = pflag.String("broker", "rabbitmq", "Broker type: rabbitmq or redis")
	brokerURI   = pflag.String("uri", "amqp://guest:guest@localhost:5672/", "Broker connection URI")
	queueName   = pflag.String("queue", "test_queue", "Queue name")
	duration    = pflag.Duration("duration", 30*time.Second, "Test duration")
	metricsFile = pflag.String("metrics-file", "consumer_metrics.json", "File to write metrics")
	logLevel    = pflag.String("log-level", "info", "Log level")
	workers     = pflag.Int("workers", 5, "Number of concurrent consumer workers")
)

func main() {
	pflag.Parse()

	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	logrus.SetLevel(level)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	ctx, cancel := context.WithTimeout(context.Background(), *duration)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logrus.Info("Received interrupt signal, stopping...")
		cancel()
	}()

	var b broker.Broker
	switch *brokerType {
	case "rabbitmq":
		b, err = broker.NewRabbitMQBroker(*brokerURI, *queueName)
	case "redis":
		b, err = broker.NewRedisBroker(*brokerURI, *queueName)
	default:
		logrus.Fatalf("Unknown broker type: %s", *brokerType)
	}
	if err != nil {
		logrus.Fatalf("Failed to create broker: %v", err)
	}
	defer b.Close()

	m := metrics.NewConsumerMetrics()
	var mu sync.Mutex 

	logrus.WithFields(logrus.Fields{
		"broker":  *brokerType,
		"queue":   *queueName,
		"workers": *workers,
	}).Info("Starting consumer")

	// Запускаем несколько воркеров
	var wg sync.WaitGroup
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			err := b.Consume(ctx, func(data []byte) error {
				parts := bytes.SplitN(data, []byte("|"), 2)
				if len(parts) < 2 {
					logrus.WithField("worker", workerID).Warn("Message missing separator")
					mu.Lock()
					m.RecordReceived()
					mu.Unlock()
					return nil
				}
				sentAt, err := time.Parse(time.RFC3339Nano, string(parts[0]))
				if err != nil {
					logrus.WithError(err).WithField("worker", workerID).Warn("Failed to parse timestamp")
				} else {
					latency := time.Since(sentAt)
					mu.Lock()
					m.RecordLatency(latency)
					mu.Unlock()
				}
				mu.Lock()
				m.RecordReceived()
				mu.Unlock()
				return nil
			})
			if err != nil && err != context.DeadlineExceeded && err != context.Canceled {
				logrus.WithError(err).WithField("worker", workerID).Error("Consumer error")
			}
		}(i)
	}

	wg.Wait()
	logrus.Info("Consumer finished")
	saveMetrics(m, *duration, *metricsFile)
}

func saveMetrics(m *metrics.ConsumerMetrics, duration time.Duration, filename string) {
	stats := m.Stats()
	data := map[string]interface{}{
		"messages_received": stats.Received,
		"errors":            stats.Errors,
		"duration_sec":      duration.Seconds(),
		"avg_latency_ms":    stats.AvgLatencyMs,
		"p95_latency_ms":    stats.P95LatencyMs,
		"max_latency_ms":    stats.MaxLatencyMs,
		"broker":            *brokerType,
	}
	file, err := os.Create(filename)
	if err != nil {
		logrus.WithError(err).Error("Failed to create metrics file")
		return
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")
	if err := enc.Encode(data); err != nil {
		logrus.WithError(err).Error("Failed to encode metrics")
	}
	logrus.WithField("file", filename).Info("Metrics saved")
}
