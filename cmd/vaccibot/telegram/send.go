package telegram

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Sender struct {
	client *http.Client
	token  string
	chatID string
}

func NewSender(token, chatID string) *Sender {
	return &Sender{
		token:  token,
		chatID: chatID,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *Sender) SendMessage(message string) error {
	silent := false
	hour := time.Now().UTC().Add(3 * time.Hour).Hour()
	if hour == 22 || (hour >= 0 && hour < 9) {
		silent = true
	}

	resp, err := s.client.Get(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s&text=%s&parse_mode=markdown&disable_notification=%v", s.token, s.chatID, url.QueryEscape(message), silent),
	)
	if err != nil {
		return fmt.Errorf("error sending telegram message: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (s *Sender) SendLocation(latitude, longitude string) error {
	resp, err := s.client.Get(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendLocation?chat_id=%s&latitude=%s&longitude=%s&disable_notification=true", s.token, s.chatID, latitude, longitude),
	)
	if err != nil {
		return fmt.Errorf("error sending telegram location: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
