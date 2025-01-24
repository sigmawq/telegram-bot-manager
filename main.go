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

	Listeners      []Listener
	ListenersMutex sync.Mutex
}

const (
	BUCKET_BOTS ef.BucketID = "bots"
)

const (
	ERROR_ID_WRONG_INPUT     = "tgb_wrong_input"
	ERROR_BOT_NOT_FOUND      = "tgb_bot_not_found"
	ERROR_BOT_ALREADY_ACTIVE = "tgb_bot_already_active"
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

	{ // Initialize database
		err = ef.NewBucket(&state.EfContext, "bots")
		if err != nil {
			panic(err)
		}
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
		ef.Iterate(bucket, func(_ ef.ID128, bot *Bot) bool {
			if bot.Listen {
				state.Listeners = append(state.Listeners, Listener{BotID: bot.ID})
			}

			return true
		})

		for i, _ := range state.Listeners {
			go BotReceiver(&state.Listeners[i])
		}

		err = tx.Commit()
		if err != nil {
			panic(err)
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
			Name:                  "bot/list",
			Handler:               ListBots,
			AuthorizationRequired: true,
		})

		ef.StartServer(&state.EfContext)

	}
}
