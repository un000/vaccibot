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
	resp, err := s.client.Get(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=%s&text=%s&parse_mode=markdown", s.token, s.chatID, url.QueryEscape(message)),
	)
	if err != nil {
		return fmt.Errorf("error sending telegram message: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

func (s *Sender) SendLocation(latitude, longitude string) error {
	resp, err := s.client.Get(
		fmt.Sprintf("https://api.telegram.org/bot%s/sendLocation?chat_id=%s&latitude=%s&longitude=%s", s.token, s.chatID, latitude, longitude),
	)
	if err != nil {
		return fmt.Errorf("error sending telegram location: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
