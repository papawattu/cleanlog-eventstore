package events

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	repo "github.com/papawattu/cleanlog-eventstore/repository"
	utils "github.com/papawattu/cleanlog-eventstore/utils"
)

type TextStream interface {
	Stream() string
	Available() bool
}

type textStream struct {
	scanner *bufio.Scanner
}

func (ts *textStream) Stream() string {
	ts.scanner.Scan()
	return ts.scanner.Text()
}

func (ts *textStream) Available() bool {
	return ts.scanner.Scan()
}

func NewTextStream(r *http.Response) TextStream {
	return &textStream{
		scanner: bufio.NewScanner(r.Body),
	}
}

type EventStreamService interface {
	EventStreamRunner()
	EventStreamHandler()
}

type EventStreamServiceImpl[T any, S comparable] struct {
	ctx             context.Context
	baseUri         string
	es              chan string
	topic           string
	lastId          string
	repo            repo.Repository[T, S]
	eventTypePrefix string
	textStream      TextStream
}

func decodeEntity[T any](data string) T {
	var wl T
	err := json.Unmarshal([]byte(data), &wl)
	if err != nil {
		log.Fatalf("Error decoding work log: %v", err)
	}
	return wl
}

func decodeEvent(ev string) Event {
	var event Event
	err := json.Unmarshal([]byte(ev), &event)
	if err != nil {
		log.Fatalf("Error decoding event: %s : %v", ev, err)
	}
	return event
}

func NewEventStreamService[T any, S comparable](ctx context.Context, baseUri string, es chan string, topic, eventTypePrefix string, ts TextStream) EventStreamService {
	return &EventStreamServiceImpl[T, S]{
		ctx:             ctx,
		baseUri:         baseUri,
		es:              es,
		topic:           topic,
		lastId:          "",
		eventTypePrefix: eventTypePrefix,
		textStream:      ts,
	}
}

func (ess *EventStreamServiceImpl[T, S]) EventStreamRunner() {

	for {
		client := utils.NewRetryableClient(10)

		req, err := http.NewRequest("GET", ess.baseUri+"/eventstream/"+ess.topic, nil)

		if err != nil {
			log.Fatalf("Error creating request: %v", err)
		}

		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Last-Event-ID", ess.lastId)

		resp, err := client.Do(req)
		if err != nil {
			log.Fatalf("Error connecting to event stream: %v", err)
			continue
		}

		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Fatalf("Error: status code %d", resp.StatusCode)
			ess.es <- "Error connecting to event stream"
		}

		if ess.textStream == nil {
			ess.textStream = NewTextStream(resp)
		}
		log.Println("Connected to event stream")

		for running := true; running; {
			select {
			case <-ess.ctx.Done():
				log.Println("Timeout")
				running = false
				break
			case <-req.Context().Done():
				log.Println("Client connection closed")
				running = false
				break
			default:
				if ess.textStream.Available() {
					e := ess.textStream.Stream()
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
						ess.es <- strings.TrimLeft(e, "data: ")
					case strings.HasPrefix(e, "id: "):
						log.Printf("Id: %s\n", strings.TrimLeft(e, "id: "))
						ess.lastId = strings.TrimLeft(e, "id: ")
					default:
						log.Printf("Unknown: %s\n", e)
					}
				}
			}
		}
	}
}

func (ess *EventStreamServiceImpl[T, S]) EventStreamHandler() {
	sha := make(map[string]string)

	for {
		ev := <-ess.es
		if ev == "" {
			log.Printf("Received empty event %+v", ess.es)
			continue
		}
		if ev == "Error connecting to event stream" {
			log.Printf("Error connecting to event stream")
		}

		log.Printf("Received event: %s", ev)

		event := decodeEvent(ev)

		if _, ok := sha[event.EventSHA]; ok {
			log.Printf("Skipping event %s", event.EventSHA)
			continue
		}

		sha[event.EventSHA] = ev

		switch event.EventType {
		case ess.eventTypePrefix + Created:
			log.Printf("Received work log created event %v", event.EventData)
			wl := decodeEntity[T](event.EventData)
			err := ess.repo.Create(ess.ctx, wl)
			log.Printf("Saved work log %v", wl)
			if err != nil {
				log.Printf("Error saving work log: %v", err)
			}
		case ess.eventTypePrefix + Deleted:
			e := decodeEntity[T](event.EventData)
			err := ess.repo.Delete(ess.ctx, e)

			if err != nil {
				log.Printf("Error deleting work log: %v", err)
			}
		case ess.eventTypePrefix + Updated:
			wl := decodeEntity[T](event.EventData)
			err := ess.repo.Save(ess.ctx, wl)
			if err != nil {
				log.Printf("Error updating work log: %v", err)
			}
		default:
			log.Printf("Unknown event type: %s", event.EventType)
		}
	}

}
