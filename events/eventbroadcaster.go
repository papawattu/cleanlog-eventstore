package events

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/papawattu/cleanlog-eventstore/repository"
	utils "github.com/papawattu/cleanlog-eventstore/utils"
)

const (
	Created      = "Created"
	Deleted      = "Deleted"
	Updated      = "Updated"
	EventUri     = "/event"
	EventVersion = 1
)

type EventBroadcaster[T any, S comparable] struct {
	repo            repository.Repository[T, S]
	broadcastUri    string
	eventTypePrefix string
}

type Event struct {
	EventType    string    `json:"eventType"`
	EventTime    time.Time `json:"eventTime"`
	EventVersion uint32    `json:"eventVersion"`
	EventSHA     string    `json:"eventSHA"`
	EventData    string    `json:"eventData"`
}

func (eb *EventBroadcaster[T, S]) postEvent(event Event) error {

	ev, err := json.Marshal(event)
	if err != nil {
		return err
	}

	client := utils.NewRetryableClient(10)

	r, err := http.NewRequest("POST", eb.broadcastUri, bytes.NewBuffer(ev))

	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")

	resp, err := client.Do(r)

	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("Error: status code %d", resp.StatusCode)
	}

	return nil
}

func (eb *EventBroadcaster[T, S]) Save(ctx context.Context, e T) error {

	var evt string

	id, err := eb.repo.GetId(ctx, e)

	if err != nil {
		return err
	}

	ok, err := eb.repo.Exists(ctx, id)

	if err != nil {
		return err
	}

	if ok {
		slog.Info("Work log already exists", "ID", id)
		evt = eb.eventTypePrefix + Updated

	} else {
		slog.Info("Work log created", "ID", id, "Event", evt)
		evt = eb.eventTypePrefix + Created
	}
	ent, err := json.Marshal(e)

	if err != nil {
		return err
	}

	h := sha256.New()

	h.Write([]byte(ent))

	// Broadcast event
	event := Event{
		EventType:    evt,
		EventTime:    time.Now(),
		EventVersion: EventVersion,
		EventSHA:     fmt.Sprintf("%x", h.Sum(nil)),
		EventData:    string(ent),
	}

	err = eb.postEvent(event)

	if err != nil {
		return err
	}

	// err = eb.repo.SaveWorkLog(wl)

	// if err != nil {

	// 	err := eb.DeleteWorkLog(*wl.WorkLogID)
	// 	if err != nil {
	// 		log.Panicf("Error saving work log: %v published rollback event", err)
	// 	}
	// 	log.Printf("Error saving work log: %v published rollback event", err)
	// 	return err
	// }

	return nil //
}

func (eb *EventBroadcaster[T, S]) Get(ctx context.Context, id S) (T, error) {
	return eb.repo.Get(ctx, id)
}

func (eb *EventBroadcaster[T, S]) GetAll(ctx context.Context) ([]T, error) {
	return eb.repo.GetAll(ctx)
}

func (eb *EventBroadcaster[T, S]) Delete(ctx context.Context, e T) error {

	wlj, err := json.Marshal(e)

	if err != nil {
		return err
	}

	h := sha256.New()

	h.Write([]byte(wlj))

	// Broadcast event
	event := Event{
		EventType:    eb.eventTypePrefix + Deleted,
		EventTime:    time.Now(),
		EventVersion: EventVersion,
		EventSHA:     fmt.Sprintf("%x", h.Sum(nil)),
		EventData:    string(wlj),
	}

	err = eb.postEvent(event)

	if err != nil {
		return err
	}

	return nil // eb.repo.DeleteWorkLog(id)
}

func (eb *EventBroadcaster[T, S]) Exists(ctx context.Context, id S) (bool, error) {
	return eb.repo.Exists(ctx, id)
}
func (eb *EventBroadcaster[T, S]) GetId(ctx context.Context, e T) (S, error) {
	return eb.repo.GetId(ctx, e)
}
func NewEventBroadcaster[T any, S comparable](ctx context.Context, repo repository.Repository[T, S], broadcastUri string, streamUri, topic string, eventTypePrefix string) *EventBroadcaster[T, S] {

	es := make(chan string)

	go EventStream(ctx, streamUri, es, topic)

	go func() {
		sha := make(map[string]string)

		for {
			ev := <-es
			if ev == "" {
				log.Printf("Received empty event %+v", es)
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
			case eventTypePrefix + Created:
				log.Printf("Received work log created event %v", event.EventData)
				wl := decodeEntity[T](event.EventData)
				err := repo.Save(ctx, wl)
				log.Printf("Saved work log %v", wl)
				if err != nil {
					log.Printf("Error saving work log: %v", err)
				}
			case eventTypePrefix + Deleted:
				e := decodeEntity[T](event.EventData)
				err := repo.Delete(ctx, e)

				if err != nil {
					log.Printf("Error deleting work log: %v", err)
				}
			case eventTypePrefix + Updated:
				wl := decodeEntity[T](event.EventData)
				err := repo.Save(ctx, wl)

				if err != nil {
					log.Printf("Error updating work log: %v", err)
				}
			}
		}

	}()

	return &EventBroadcaster[T, S]{
		repo:            repo,
		broadcastUri:    broadcastUri + "/event/" + topic,
		eventTypePrefix: eventTypePrefix,
	}
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
