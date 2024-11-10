package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func main() {
	port := os.Getenv("PORT")
	bootstrapServers := os.Getenv("BOOTSTRAP_SERVERS")

	if port == "" {
		port = "3000"
	}

	if bootstrapServers == "" {
		bootstrapServers = "localhost:29092,localhost:39092,localhost:49092"
	} else {
		bootstrapServers = os.Getenv("BOOTSTRAP_SERVERS")
	}

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": bootstrapServers,
		"client.id":         "myProducer",
		"acks":              "all"})

	if err != nil {
		log.Fatal(err)
	}

	defer p.Close()

	deliveryChan := make(chan kafka.Event)

	topic := "topic"

	http.HandleFunc("/event/{topic}", func(w http.ResponseWriter, r *http.Request) {

		if r.PathValue("topic") != "" {
			topic = r.PathValue("topic")
		}
		if r.Method != http.MethodPost {
			http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
			return
		}

		event, err := io.ReadAll(r.Body)

		if err != nil {
			http.Error(w, "Failed to read request body", http.StatusInternalServerError)
			return
		}

		err = p.Produce(&kafka.Message{
			TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
			Value:          event,
		}, deliveryChan)
		if err != nil {
			log.Printf("Failed to produce message: %v", err)
		}

		log.Printf("Produced message: %v", event)
		e := <-deliveryChan
		m := e.(*kafka.Message)
		if m.TopicPartition.Error != nil {
			log.Printf("Delivery failed: %v", m.TopicPartition.Error)
		} else {
			log.Printf("Delivered message to topic %s [%d] at offset %v", *m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)
			w.WriteHeader(http.StatusCreated)

		}

	})

	log.Printf("Listening on port :%s", port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", port), nil); err != nil {
		log.Fatal(err)
	}
}
