package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func main() {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": "localhost:39092,localhost:49092,localhost:29092",
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

	port := flag.String("port", "8080", "port to listen on")
	flag.Parse()

	log.Printf("Listening on port %s", *port)
	if err := http.ListenAndServe(fmt.Sprintf(":%s", *port), nil); err != nil {
		log.Fatal(err)
	}
}
