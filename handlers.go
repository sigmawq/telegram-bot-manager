package main

func CringeBotHandler(listener *Listener) {
	select {
	case updates, _ := <-listener.In:
		for _, update := range updates {
			responseText := "I don't understand you!"
			if update.Message.Text == "hello" {
				responseText = "world"
			}

			listener.Out <- TelegramSendMessage{
				ChatID: update.Message.From.ID,
				Text:   responseText,
			}
		}
	default:
	}
}
