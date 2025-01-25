package main

import (
	ef "github.com/sigmawq/easyframework"
	"log"
	"net/http"
	"sync"
)

func Authorization(ctx *ef.RequestContext, w http.ResponseWriter, r *http.Request) bool {
	return true
}

var state struct {
	EfContext ef.Context

	Listeners      []*Listener
	ListenersMutex sync.Mutex

	BotHandlers map[HandlerID]func(*Listener)
}

const (
	BUCKET_BOTS ef.BucketID = "bots"
)

const (
	ERROR_ID_WRONG_INPUT        = "tgb_wrong_input"
	ERROR_BOT_NOT_FOUND         = "tgb_bot_not_found"
	ERROR_BOT_ALREADY_ACTIVE    = "tgb_bot_already_active"
	ERROR_BOT_HANDLER_NOT_FOUND = "tgb_bot_handler_not_found"
	ERROR_BOT_LISTENS           = "tgb_bot_listens"
	ERROR_HANDLER_IS_EMPTY      = "tgb_handler_is_empty"
)

func main() {
	params := ef.InitializeParams{
		Port:          6500,
		StdoutLogging: true,
		FileLogging:   false,
		DatabasePath:  "db",
		Authorization: Authorization,
	}

	err := ef.Initialize(&state.EfContext, params)
	if err != nil {
		log.Println("Error while initializing EF:", err)
		return
	}

	{ // Basic initialization
		state.BotHandlers = make(map[HandlerID]func(*Listener))
	}

	{ // Initialize database
		err = ef.NewBucket(&state.EfContext, "bots")
		if err != nil {
			panic(err)
		}
	}
	{ // Initialize handlers
		RegisterBotHandler(HANDLER_ID_CRINGE_BOT, CringeBotHandler)
	}

	{ // Initialize listeners
		tx, err := ef.WriteTx(&state.EfContext)
		if err != nil {
			log.Fatal(err)
		}

		bucket, err := ef.GetBucket(tx, BUCKET_BOTS)
		if err != nil {
			log.Fatal(err)
		}

		botsToStart := make([]ef.ID128, 0)
		ef.Iterate(bucket, func(id ef.ID128, _ *Bot) bool {
			botsToStart = append(botsToStart, id)
			return true
		})

		err = tx.Commit()
		if err != nil {
			panic(err)
		}

		for _, id := range botsToStart {
			_StartBot(id, true)
		}
	}

	{ // Initialize server
		ef.NewRPC(&state.EfContext, ef.NewRPCParams{
			Name:                  "bot/add",
			Handler:               AddBot,
			AuthorizationRequired: true,
		})

		ef.NewRPC(&state.EfContext, ef.NewRPCParams{
			Name:                  "bot/start",
			Handler:               StartBot,
			AuthorizationRequired: true,
		})

		ef.NewRPC(&state.EfContext, ef.NewRPCParams{
			Name:                  "bot/stop",
			Handler:               StopBot,
			AuthorizationRequired: true,
		})

		ef.NewRPC(&state.EfContext, ef.NewRPCParams{
			Name:                  "bot/setHandler",
			Handler:               SetBotHandler,
			AuthorizationRequired: true,
		})

		ef.NewRPC(&state.EfContext, ef.NewRPCParams{
			Name:                  "bot/list",
			Handler:               ListBots,
			AuthorizationRequired: true,
		})

		ef.StartServer(&state.EfContext)

	}
}
