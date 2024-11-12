package events

import (
	"bufio"
	"context"
	"log"
	"net/http"
	"strings"

	utils "github.com/papawattu/cleanlog-eventstore/utils"
)

func EventStream(ctx context.Context, baseUri string, es chan string, topic string) {
	lastId := ""

	for {
		client := utils.NewRetryableClient(10)

		req, err := http.NewRequest("GET", baseUri+"/eventstream/"+topic, nil)

		if err != nil {
			log.Fatalf("Error creating request: %v", err)
		}

		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Last-Event-ID", lastId)

		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error connecting to event stream: %v", err)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Error: status code %d", resp.StatusCode)
			es <- "Error connecting to event stream"
		}

		log.Println("Connected to event stream")
		scanner := bufio.NewScanner(resp.Body)

		for running := true; running; {
			select {
			case <-ctx.Done():
				log.Println("Timeout")
				running = false
				break
			case <-req.Context().Done():
				log.Println("Client connection closed")
				running = false
				break
			default:
				scanner.Scan()
				e := scanner.Text()
				if e == "" {
					log.Println("Empty event")
					running = false
					break
				}

				switch {
				case strings.HasPrefix(e, "event: "):
					log.Printf("Event: %s\n", strings.TrimLeft(e, "event: "))
				case strings.HasPrefix(e, "data: "):
					log.Printf("Data: %s\n", strings.TrimLeft(e, "data: "))
					es <- strings.TrimLeft(e, "data: ")
				case strings.HasPrefix(e, "id: "):
					log.Printf("Id: %s\n", strings.TrimLeft(e, "id: "))
					scanner.Scan()
					scanner.Text()
					lastId = strings.TrimLeft(e, "id: ")
				default:
					log.Printf("Unknown: %s\n", e)
				}
			}
		}
	}
}
