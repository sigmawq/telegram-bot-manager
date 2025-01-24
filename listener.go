package main

import (
	"encoding/json"
	"fmt"
	ef "github.com/sigmawq/easyframework"
	"io"
	"log"
	"net/http"
	"time"
)

type Listener struct {
	ArrayIndex     int
	BotID          ef.ID128
	ReceiveMessage func(string) string
	NeedsToStop    bool
}

type TelegramUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

type TelegramMessage struct {
	MessageID       int `json:"message_id"`
	MessageThreadID int `json:"message_thread_id"`
}

type TelegramUpdate struct {
	UpdateID int             `json:"update_id"`
	Message  TelegramMessage `json:"message"`
}

func BotReceiver(listener *Listener) {
	log.Printf("Begin listener for BotID %v", listener.BotID)

	var lastUpdateID int
	var lastTelegramCall time.Time
	for {
		if listener.NeedsToStop {
			log.Printf("Stopping bot ID %v receiver..", listener.BotID)
			return
		}

		bot, err := ef.GetByID[Bot](state.EfContext, BUCKET_BOTS, listener.BotID)
		if err != nil {
			log.Println(err)
			continue
		}

		maxRps := 1 // TODO: Change
		minimumDelayMs := int(1000 / maxRps)
		sinceLastCall := int(time.Now().Sub(lastTelegramCall).Milliseconds())
		if sinceLastCall < minimumDelayMs {
			time.Sleep(time.Millisecond * time.Duration(minimumDelayMs-sinceLastCall))
		}

		requestString := fmt.Sprintf("https://api.telegram.org/bot%v/getUpdates?offset=%v", bot.APIKey, lastUpdateID)
		response, err := http.Get(requestString)
		lastTelegramCall = time.Now()
		if err != nil {
			log.Println(err)
			continue
		}

		type Updates struct {
			Result []TelegramUpdate `json:"result"`
		}

		bytes, _ := io.ReadAll(response.Body)
		var updates Updates
		json.Unmarshal(bytes, &updates)

		for _, update := range updates.Result {
			log.Printf("%#v", update)
			lastUpdateID = update.UpdateID + 1
		}
	}
}
