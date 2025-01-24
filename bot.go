package main

import (
	ef "github.com/sigmawq/easyframework"
	"log"
)

type BotHandlerID string

const (
	BOT_HANDLER_CRINGE_BOT BotHandlerID = "cringe_bot"
)

type Bot struct {
	ID        ef.ID128     `id:"1"`
	Name      string       `id:"2"`
	APIKey    string       `id:"3"`
	Listen    bool         `id:"4"`
	HandlerID BotHandlerID `id:"5"`
	MaxRPS    float64      `id:"6"`
}

type AddBotRequest struct {
	Name   string
	APIKey string
}

type AddBotResponse struct {
	BotID ef.ID128
}

func AddBot(ctx *ef.RequestContext, request AddBotRequest) (response AddBotResponse, problem ef.Problem) {
	if request.Name == "" || request.APIKey == "" {
		problem.ErrorID = ERROR_ID_WRONG_INPUT
		problem.Message = "Bot name required"
		return
	}

	tx, err := ef.WriteTx(&state.EfContext)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}
	defer tx.Rollback()

	bucket, err := ef.GetBucket(tx, BUCKET_BOTS)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	botID := ef.NewID128()
	log.Println("on add id", botID[:])
	bot := Bot{
		ID:     botID,
		Name:   request.Name,
		APIKey: request.APIKey,
	}
	err = ef.Insert(bucket, botID, bot)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	tx.Commit()

	log.Printf("create bot bot:%#v", bot)
	return
}

func ListBots(ctx *ef.RequestContext) (result []Bot, problem ef.Problem) {
	tx, _ := ef.ReadTx(&state.EfContext)
	bucket, err := ef.GetBucket(tx, BUCKET_BOTS)
	if err != nil {
		problem.ErrorID = ef.ERROR_INTERNAL
	}
	ef.Iterate(bucket, func(id ef.ID128, bot *Bot) bool {
		result = append(result, *bot)
		return true
	})

	return
}

type StartBotRequest struct {
	BotID ef.ID128
	Value int
}

func StartBot(ctx *ef.RequestContext, request StartBotRequest) (problem ef.Problem) {
	log.Println("bot id on start bot", request.BotID[:])

	bot, err := ef.GetByID[Bot](state.EfContext, BUCKET_BOTS, request.BotID)
	if err != nil {
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	if bot == nil {
		problem.ErrorID = ERROR_BOT_NOT_FOUND
		return
	}

	if bot.Listen {
		problem.ErrorID = ERROR_BOT_ALREADY_ACTIVE
		return
	}

	bot.Listen = true

	state.ListenersMutex.Lock()
	defer state.ListenersMutex.Unlock()

	state.Listeners = append(state.Listeners, Listener{
		BotID: bot.ID,
	})
	listener := &state.Listeners[len(state.Listeners)-1]
	go BotReceiver(listener)

	err = ef.InsertByID(state.EfContext, BUCKET_BOTS, request.BotID, *bot)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	return
}
