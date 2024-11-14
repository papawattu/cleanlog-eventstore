package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	repo "github.com/papawattu/cleanlog-eventstore/repository"
)

const (
	Created      = "Created"
	Deleted      = "Deleted"
	Updated      = "Updated"
	EventUri     = "/event"
	EventVersion = 1
)

type DeliverEvent[T any, S comparable] func(eb *EventService[T, S], event Event) error

type EventService[T any, S comparable] struct {
	postEvent       DeliverEvent[T, S]
	repo            repo.Repository[T, S]
	broadcastUri    string
	eventTypePrefix string
	HttpTransport   Transport
}

type Event struct {
	EventType    string    `json:"eventType"`
	EventTime    time.Time `json:"eventTime"`
	EventVersion uint32    `json:"eventVersion"`
	EventSHA     string    `json:"eventSHA"`
	EventData    string    `json:"eventData"`
}

func (eb *EventService[T, S]) Create(ctx context.Context, e T) error {

	slog.Info("EventBroadcaster", "Save", e)
	ent, err := json.Marshal(e)

	if err != nil {
		return err
	}

	// Broadcast event
	event := Event{
		EventType:    eb.eventTypePrefix + Created,
		EventTime:    time.Now(),
		EventVersion: EventVersion,
		EventData:    string(ent),
	}

	slog.Info("EventBroadcaster", "Create", event.EventData)
	err = eb.HttpTransport.PostEvent(event)

	if err != nil {
		slog.Error("Error broadcasting event", "error", err)
		return err
	}

	slog.Info("EventBroadcaster", "Create", "Event published")

	return nil
}

func (eb *EventService[T, S]) Save(ctx context.Context, e T) error {

	slog.Info("EventBroadcaster", "Save", e)
	ent, err := json.Marshal(e)

	if err != nil {
		return err
	}

	// Broadcast event
	event := Event{
		EventType:    eb.eventTypePrefix + Updated,
		EventTime:    time.Now(),
		EventVersion: EventVersion,
		EventData:    string(ent),
	}

	slog.Info("EventBroadcaster", "Save", event.EventData)
	err = eb.HttpTransport.PostEvent(event)

	if err != nil {
		slog.Error("Error broadcasting event", "error", err)
		return err
	}

	slog.Info("EventBroadcaster", "Save", "Event published")

	return nil
}

func (eb *EventService[T, S]) Get(ctx context.Context, id S) (T, error) {
	return eb.repo.Get(ctx, id)
}

func (eb *EventService[T, S]) GetAll(ctx context.Context) ([]T, error) {
	return eb.repo.GetAll(ctx)
}

func (eb *EventService[T, S]) Delete(ctx context.Context, e T) error {

	wlj, err := json.Marshal(e)

	if err != nil {
		return err
	}

	// Broadcast event
	event := Event{
		EventType:    eb.eventTypePrefix + Deleted,
		EventTime:    time.Now(),
		EventVersion: EventVersion,
		EventData:    string(wlj),
	}

	err = eb.HttpTransport.PostEvent(event)

	if err != nil {
		return err
	}

	return nil // eb.repo.DeleteWorkLog(id)
}

func (eb *EventService[T, S]) Exists(ctx context.Context, id S) (bool, error) {
	return eb.repo.Exists(ctx, id)
}

func (eb *EventService[T, S]) GetId(ctx context.Context, e T) (S, error) {
	return eb.repo.GetId(ctx, e)
}

func NewEventService[T any, S comparable](ctx context.Context, repo repo.Repository[T, S],
	broadcastUri string,
	streamUri, topic string,
	eventTypePrefix string,
	postEvent DeliverEvent[T, S],
	transport Transport) *EventService[T, S] {

	es := make(chan string)

	if transport == nil {
		transport = NewHttpTransport(broadcastUri)
	}

	ess := NewEventStreamService[T, S](ctx, streamUri, es, topic, eventTypePrefix, nil)

	go ess.EventStreamRunner()

	go ess.EventStreamHandler()

	return &EventService[T, S]{
		repo:            repo,
		broadcastUri:    broadcastUri + "/event/" + topic,
		eventTypePrefix: eventTypePrefix,
		HttpTransport:   transport,
	}
}
