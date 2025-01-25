package main

import (
	"encoding/json"
	"fmt"
	ef "github.com/sigmawq/easyframework"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type Listener struct {
	ArrayIndex int
	BotID      ef.ID128

	UserOperation                   func(*Listener)
	UserOperationPeriodMilliseconds float64

	NeedsToStop bool

	In  chan []TelegramUpdate
	Out chan TelegramSendMessage
}

type TelegramChat struct {
	Id int `json:"id"`
}

type TelegramUser struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Chat      string `json:"chat"`
}

type TelegramMessage struct {
	MessageID int          `json:"message_id"`
	From      TelegramUser `json:"from"`
	Text      string       `json:"text"`
}

type TelegramUpdate struct {
	UpdateID int             `json:"update_id"`
	Message  TelegramMessage `json:"message"`
}

type TelegramSendMessage struct {
	ChatID int    `json:"chat_id"`
	Text   string `json:"text"`
}

func BotReceiver(listener *Listener) {
	log.Printf("[receiver] Start listener for BotID %v", listener.BotID)

	var lastUpdateID int
	var lastTelegramCall time.Time
	for {
		if listener.NeedsToStop {
			log.Printf("[receiver] stopping %v", listener.BotID)
			return
		}

		bot, err := ef.GetByID[Bot](&state.EfContext, BUCKET_BOTS, listener.BotID)
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

func BotSender(listener *Listener) {
	log.Printf("[sender] Start BotID %v sender", listener.BotID)

	var lastUpdateID int
	var lastTelegramCall time.Time
	for {
		if listener.NeedsToStop {
			log.Printf("[sender] stopping %v", listener.BotID)
			return
		}

		bot, err := ef.GetByID[Bot](&state.EfContext, BUCKET_BOTS, listener.BotID)
		if err != nil {
			log.Println(err)
			continue
		}

		maxRps := 1
		minimumDelayMs := int(1000 / maxRps)
		sinceLastCall := int(time.Now().Sub(lastTelegramCall).Milliseconds())
		if sinceLastCall < minimumDelayMs {
			time.Sleep(time.Millisecond * time.Duration(minimumDelayMs-sinceLastCall))
		}

		var message TelegramSendMessage
		var ok bool
		select {
		case message, ok = <-listener.Out:
			if !ok {
				continue
			}
		default:
			
		}

		requestBody, _ := json.Marshal(message)
		requestString := fmt.Sprintf("https://api.telegram.org/bot%v/sendMessage", bot.APIKey, lastUpdateID)
		response, err := http.Post(requestString, "application/json", strings.NewReader(string(requestBody)))
		lastTelegramCall = time.Now()
		if err != nil {
			log.Println(err)
			continue
		}

		if response.StatusCode != 200 {
			body, _ := io.ReadAll(response.Body)
			log.Println("[Bot %v] Failed to send message: status_code:%v reason:%v", bot.ID, response.StatusCode, string(body))
		}
	}
}

func BotUserOperationRunner(listener *Listener) {
	if listener.UserOperation == nil {
		log.Printf("[user operations runner] No user handler for bot id %v", listener.BotID)
		return
	}

	var lastCall time.Time
	for {
		if listener.NeedsToStop {
			log.Printf("[user operations runner] stopping %v", listener.BotID)
			return
		}

		periodMilliseconds := 1.0
		if listener.UserOperationPeriodMilliseconds < 1 {
			periodMilliseconds = 1
		} else {
			periodMilliseconds = listener.UserOperationPeriodMilliseconds
		}
		sinceLastCall := float64(time.Now().Sub(lastCall).Milliseconds())
		if sinceLastCall < periodMilliseconds {
			diff := periodMilliseconds - sinceLastCall
			time.Sleep(time.Millisecond * time.Duration(diff))
		}

		lastCall = time.Now()
		listener.UserOperation(listener)
	}
}
