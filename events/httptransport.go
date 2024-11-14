package events

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"

	utils "github.com/papawattu/cleanlog-eventstore/utils"
)

type Transport interface {
	PostEvent(event Event) error
}
type HttpTransport struct {
	uri string
}

func (ht *HttpTransport) PostEvent(event Event) error {

	ev, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h := sha256.New()

	h.Write([]byte(ev))

	event.EventSHA = fmt.Sprintf("%x", h.Sum(nil))

	ev, err = json.Marshal(event)

	if err != nil {
		return err
	}

	client := utils.NewRetryableClient(10)

	r, err := http.NewRequest("POST", ht.uri, bytes.NewBuffer(ev))

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

func NewHttpTransport(uri string) Transport {
	return &HttpTransport{
		uri: uri,
	}
}
