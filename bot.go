package main

import (
	ef "github.com/sigmawq/easyframework"
	"log"
)

type HandlerID string

const (
	HANDLER_ID_CRINGE_BOT = "handler_cringe_bot"
)

type Bot struct {
	ID               ef.ID128  `id:"1"`
	Name             string    `id:"2"`
	APIKey           string    `id:"3"`
	Listen           bool      `id:"4"`
	HandlerID        HandlerID `id:"5"`
	MaxSendRPS       float64   `id:"6"`
	MaxGetUpdatesRPS float64   `id:"7"`
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

func RegisterBotHandler(id HandlerID, procedure func(*Listener)) {
	_, occupied := state.BotHandlers[id]
	if occupied {
		log.Printf("-> %v", id)
		panic("HandlerID already in use!")
	}
}

type StartBotRequest struct {
	BotID ef.ID128
	Value int
}

func StartBot(ctx *ef.RequestContext, request StartBotRequest) (problem ef.Problem) {
	return _StartBot(request.BotID)
}

func _StartBot(BotID ef.ID128) (problem ef.Problem) {
	bot, err := ef.GetByID[Bot](&state.EfContext, BUCKET_BOTS, BotID)
	if err != nil {
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	if bot == nil {
		problem.ErrorID = ERROR_BOT_NOT_FOUND
		return
	}

	if bot.Listen { // Bot is already running so it's fine
		return
	}

	bot.Listen = true

	state.ListenersMutex.Lock()
	defer state.ListenersMutex.Unlock()

	state.Listeners = append(state.Listeners, &Listener{
		BotID:         bot.ID,
		UserOperation: state.BotHandlers[bot.HandlerID],
	})
	listener := state.Listeners[len(state.Listeners)-1]
	go BotReceiver(listener)
	go BotSender(listener)
	go BotUserOperationRunner(listener)

	err = ef.InsertByID(&state.EfContext, BUCKET_BOTS, BotID, *bot)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	return
}

type StopBotRequest struct {
	BotID ef.ID128
}

func StopBot(ctx *ef.RequestContext, request StopBotRequest) (problem ef.Problem) {
	problem.ErrorID = _StopBot(request.BotID)
	return
}

func _StopBot(botID ef.ID128) ef.ErrorID {
	state.ListenersMutex.Lock()
	defer state.ListenersMutex.Unlock()

	bot, err := ef.GetByID[Bot](&state.EfContext, BUCKET_BOTS, botID)
	if err != nil {
		return ef.ERROR_INTERNAL
	}
	bot.Listen = false
	err = ef.InsertByID(&state.EfContext, BUCKET_BOTS, bot.ID, *bot)
	if err != nil {
		return ef.ERROR_INTERNAL
	}

	listener_i, found := ef.SearchI(state.Listeners, func(listener *Listener) bool {
		return listener.BotID == botID
	})
	if found {
		listener := state.Listeners[listener_i]
		listener.NeedsToStop = true
	}
	ef.Remove(state.Listeners, listener_i)

	return ef.ERROR_NONE
}

type SetBotHandlerRequest struct {
	BotID     ef.ID128
	HandlerID HandlerID
}

func SetBotHandler(ctx *ef.RequestContext, request SetBotHandlerRequest) (problem ef.Problem) {
	if request.HandlerID == "" {
		problem.ErrorID = ERROR_HANDLER_IS_EMPTY
		return
	}

	bot, err := ef.GetByID[Bot](&state.EfContext, BUCKET_BOTS, request.BotID)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	if bot == nil {
		problem.ErrorID = ERROR_BOT_NOT_FOUND
		return
	}
	if bot.Listen {
		problem.ErrorID = ERROR_BOT_LISTENS
		problem.Message = "Cannot change handler while bot is listening"
		return
	}

	// We are not validating this handler because it is on the user to check handler and assign required procedure
	oldHandler := request.HandlerID
	bot.HandlerID = request.HandlerID
	err = ef.InsertByID(&state.EfContext, BUCKET_BOTS, bot.ID, *bot)
	if err != nil {
		log.Println(err)
		problem.ErrorID = ef.ERROR_INTERNAL
		return
	}

	log.Printf("bot_id:%v old_handler:%v new_handler:%v", bot.ID, oldHandler, bot.HandlerID)

	return
}
