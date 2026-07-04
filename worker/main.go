package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
)

type ClickEvent struct {
	ShortCode string    `json:"short_code"`
	IPAddress string    `json:"ip_address"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	ctx := context.Background()

	// 1. Connect to Postgres
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/ledger?sslmode=disable"
	}
	dbPool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer dbPool.Close()

	// 2. Connect to RabbitMQ (with retry logic)
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	for i := 0; i < 5; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("RabbitMQ not ready, retrying...")
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a channel: %v", err)
	}
	defer ch.Close()

	// 3. Consume from the Queue
	msgs, err := ch.Consume(
		"click_events", // queue name
		"",             // consumer
		false,          // auto-ack (we will manually acknowledge to ensure no data loss)
		false,          // exclusive
		false,          // no-local
		false,          // no-wait
		nil,            // args
	)
	if err != nil {
		log.Fatalf("Failed to register a consumer: %v", err)
	}

	log.Println("Worker started. Waiting for click events...")

	// Listen for messages forever
	for msg := range msgs {
		var event ClickEvent
		if err := json.Unmarshal(msg.Body, &event); err != nil {
			log.Printf("Error decoding JSON: %v", err)
			msg.Nack(false, false) // Drop bad messages
			continue
		}

		query := `INSERT INTO clicks (short_code, ip_address, clicked_at) VALUES ($1, $2, $3)`
		_, err := dbPool.Exec(ctx, query, event.ShortCode, event.IPAddress, event.Timestamp)
		if err != nil {
			log.Printf("Failed to insert into DB: %v", err)
			msg.Nack(false, true) // Requeue the message if the DB fails
			continue
		}

		log.Printf("Logged click for code: %s", event.ShortCode)
		msg.Ack(false) // Manually acknowledge success so RabbitMQ deletes the message
	}
}
