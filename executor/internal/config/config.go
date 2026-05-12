package config

import (
	"os"
)

type Config struct {
	ExecutorID       string
	ExecutorName     string
	SchedulerAddr    string
	Capacity         int32
}

func Load() *Config {
	executorID := getEnv("EXECUTOR_ID", "executor-1")
	return &Config{
		ExecutorID:    executorID,
		ExecutorName:  getEnv("EXECUTOR_NAME", executorID),
		SchedulerAddr: getEnv("SCHEDULER_ADDR", "localhost:50051"),
		Capacity:      10,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
