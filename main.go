package main

import (
	"bufio"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type appConfig struct {
	Bot botConfig `yaml:"bot"`
}

type botConfig struct {
	Token string `yaml:"token"`
}

type App struct {
	config     *appConfig
	tg_bot     *telegram
	proc       *processor
	usrLangMap map[int64]string
}

type RpcReq struct {
	Audio string `json:"audio"`
	Lang  string `json:"lang"`
}
type SendMsg struct {
	Text string `json:"text,omitempty"`
	Success bool `json:"success"`
}

func (app *App) StartBot() {
	app.tg_bot = newTelegram(app.config.Bot.Token)
	app.proc = newProcessor()
	defer app.stop()

	// Pass cancellable context to goroutine
	go app.receiveUpdates()
	// Tell the user the bot is online
	log.Println("Start listening for updates. Press enter to stop")

	// Wait for a newline symbol, then cancel handling updates
	bufio.NewReader(os.Stdin).ReadBytes('\n')
}

func (app *App) receiveUpdates() {
	// `for {` means the loop is infinite until we manually stop it
	for {
		select {
		// stop looping if ctx is cancelled
		case <-app.tg_bot.TelegramContext.Done():
			return
		// receive update from channel and then handle it
		case update := <-app.tg_bot.TelegramMessageChannel:
			log.Println(update.Message.Text)
			//
			if update.Message.Voice != nil {
				var voiceBytes = app.tg_bot.getFile(update.Message.Voice.FileID)
				log.Println("Received " + fmt.Sprint(binary.Size(voiceBytes)) + "bytes.")

				langCode, ok := app.usrLangMap[update.Message.Chat.ID]
				if !ok {
					langCode = "EN"
				}
				msg := RpcReq{b64.StdEncoding.EncodeToString(voiceBytes), langCode}
				b, _ := json.Marshal(msg)
				app.proc.sendVoiceToTextReq(b, update.Message.Chat.ID)

				app.tg_bot.send("your request is being processed", update.Message.Chat.ID)
				// failOnError(err, "Failed to send msg to user")
			}
			if update.Message.IsCommand() {
				log.Println("command: " + update.Message.Command())
				switch update.Message.Command() {
				case "RU":
					app.usrLangMap[update.Message.Chat.ID] = "RU"
				case "EN":
					app.usrLangMap[update.Message.Chat.ID] = "EN"
				}
			}
		case d := <-app.proc.mq_consumer:
			chatID, text := app.proc.processReqResult(d)

			app.tg_bot.send(text, chatID)
			// failOnError(err, "Failed to send msg to user")
		}
	}
}

func (app *App) stop() {
	app.tg_bot.stop()
	app.proc.stop()
}

// config
func readConf(filename string) (*appConfig, error) {
	buf, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	c := &appConfig{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filename, err)
	}

	return c, err
}

// rabbitmq
func failOnError(err error, msg string) {
	if err != nil {
		log.Panicf("%s: %s", msg, err)
	}
}

func main() {
	config, err := readConf("config.yaml")
	if err != nil {
		log.Fatal(err)
	}

	a := App{config, nil, nil, make(map[int64]string)}
	a.StartBot()
}
