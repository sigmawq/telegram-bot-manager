package main

import (
	ef "github.com/sigmawq/easyframework"
	"log"
	"net/http"
)

func Authorization(ctx *ef.RequestContext, w http.ResponseWriter, r *http.Request) bool {
	return true
}

var state struct {
	EfContext ef.Context
}

type Bot struct {
	ID     ef.ID128 `id:"1"`
	Name   string   `id:"2"`
	APIKey string   `id:"3"`
}

const (
	BUCKET_BOTS ef.BucketID = "bots"
)

const (
	ERROR_ID_WRONG_INPUT = "tgb_wrong_input"
)

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

	err = ef.NewBucket(&state.EfContext, "bots")
	if err != nil {
		panic(err)
	}

	ef.NewRPC(&state.EfContext, ef.NewRPCParams{
		Name:                  "bot/add",
		Handler:               AddBot,
		AuthorizationRequired: true,
	})

	ef.NewRPC(&state.EfContext, ef.NewRPCParams{
		Name:                  "bot/list",
		Handler:               ListBots,
		AuthorizationRequired: true,
	})

	ef.StartServer(&state.EfContext)
}
