package commands

import (
	"errors"
	"fmt"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/thesunwave/pososyamba_bot/internal/app/analytics"
	"github.com/thesunwave/pososyamba_bot/internal/app/cache"
	"github.com/thesunwave/pososyamba_bot/internal/app/fakenews"
	"github.com/thesunwave/pososyamba_bot/internal/app/string_builder"

	"io/ioutil"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

var top_phrase = "Однажды один очень мудрый человек сказал: ты пидор. Помню на одного наткнулись в музее компьютерных игр, работает там."

type RequiredParams struct {
	Update        *tgbotapi.Update
	StringBuilder *string_builder.StringBuilder
	Config        *viper.Viper
}

func (params RequiredParams) Start() *[]tgbotapi.MessageConfig {
	var messages []tgbotapi.MessageConfig

	message := params.Update.Message

	msg := tgbotapi.NewMessage(message.Chat.ID, "")
	msg.Text = "Hi there. That bot can make some things:\n" +
		"/pososyamba - send pososyamba\n" +
		"/f - Press F to pay respect\n" +
		"/gay_id - Know your gay_id\n" +
		"/renew_gay_id - Renew your gay_id\n" +
		"/hot_news - Hot news with Markov's chains and Meduza's headings\n" +
		"/mrkshi - send MRKSHI"

	go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", "start")

	messages = append(messages, msg)

	return &messages
}

func (params RequiredParams) Pososyamba() *[]tgbotapi.MessageConfig {
	var messages []tgbotapi.MessageConfig

	message := params.Update.Message

	preparedPhrases := params.Config.GetStringSlice("prepared_phrases")

	msg := tgbotapi.NewMessage(params.Update.Message.Chat.ID, "")
	msg.Text = preparedPhrases[rand.Intn(len(preparedPhrases))]
	messages = append(messages, msg)

	go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", "pososyamba")

	msg = tgbotapi.NewMessage(params.Update.Message.Chat.ID, "")
	msg.Text = params.StringBuilder.BuildPososyamba()
	messages = append(messages, msg)

	return &messages
}

func (params RequiredParams) GayID() *[]tgbotapi.MessageConfig {
	return getID("gay_id", &params)
}

func (params RequiredParams) MrazID() *[]tgbotapi.MessageConfig {
	return getID("mraz_id", &params)
}

func getID(message_type string, params *RequiredParams) *[]tgbotapi.MessageConfig {
	var messages []tgbotapi.MessageConfig
	var gayID, username string
	var clientID int
	var err error

	message := params.Update.Message

	msg := tgbotapi.NewMessage(params.Update.Message.Chat.ID, "")

	forwardedMessage := params.Update.Message

	if forwardedMessage.ReplyToMessage != nil {
		username = params.StringBuilder.FormattedUsername(forwardedMessage.ReplyToMessage)
		clientID = forwardedMessage.ReplyToMessage.From.ID
		gayID, err = cache.Redis().Get(strconv.Itoa(clientID)).Result()

		log.Info().Str("ClientID", string(clientID))
		msg.ReplyToMessageID = forwardedMessage.ReplyToMessage.MessageID
		log.Info().Str("ClientID", gayID)
	} else {
		username = params.StringBuilder.FormattedUsername(forwardedMessage)
		clientID = forwardedMessage.From.ID
		gayID, err = cache.Redis().Get(strconv.Itoa(clientID)).Result()
		log.Info().Str("ClientID", string(clientID))
		log.Info().Str("ClientID", gayID)
	}
	// TODO: Add a handler to have an opportunity to distinguish empty key from other errors
	if err != nil {
		msg.Text = params.StringBuilder.GenerateGayID()

		err := cache.Redis().Set(strconv.Itoa(clientID), msg.Text, 0).Err()

		if err != nil {
			log.Error().Err(err)
		}
	} else {
		msg.Text = gayID
	}

	msg.Text = username + " has " + message_type + ": #" + msg.Text

	go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", message_type)

	messages = append(messages, msg)

	return &messages
}

func (params RequiredParams) RenewGayID() *[]tgbotapi.MessageConfig {
	var messages []tgbotapi.MessageConfig

	message := params.Update.Message

	msg := tgbotapi.NewMessage(message.Chat.ID, "")

	gayID := params.StringBuilder.GenerateGayID()

	log.Info().Str("GAY ID:", gayID)

	msg.Text = params.StringBuilder.FormattedUsername(message) + " you have updated gay_id: #" + gayID

	err := cache.Redis().Set(strconv.Itoa(message.From.ID), gayID, 0).Err()

	if err != nil {
		log.Error().Err(err)
	}

	go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", "renew_gay_id")

	messages = append(messages, msg)

	return &messages
}

func (params RequiredParams) HotNews() *[]tgbotapi.MessageConfig {
	var messages []tgbotapi.MessageConfig

	message := params.Update.Message

	news, err := fakenews.FetchTitle()

	if err != nil {
		log.Error().Err(err)
		return &messages
	}

	msg := tgbotapi.NewMessage(message.Chat.ID, "")
	msg.Text = news

	go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", "hot_news")

	messages = append(messages, msg)

	return &messages
}

func (params RequiredParams) F() *[]tgbotapi.AnimationConfig {
	var messages []tgbotapi.AnimationConfig

	message := params.Update.Message

	gif, name, err := dancersFile()
	if err != nil {
		log.Error().Err(err).Msg("")
		return &messages
	}

	fileBytes := tgbotapi.FileBytes{
		Name:  name,
		Bytes: gif,
	}

	msg := tgbotapi.NewAnimationUpload(message.Chat.ID, fileBytes)

	if message.ReplyToMessage != nil {
		msg.ReplyToMessageID = message.ReplyToMessage.MessageID
	}

	go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", "F")

	messages = append(messages, msg)

	return &messages
}

func dancersFile() ([]byte, string, error) {
	fileInfo, err := ioutil.ReadDir("assets/dancers")
	if err != nil {
		log.Error().Err(err)
		return []byte{}, "", err
	}

	var onlyImages []os.FileInfo

	for _, f := range fileInfo {
		if strings.Split(f.Name(), ".")[1] == "gif" {
			onlyImages = append(onlyImages, f)
		}
	}

	if len(onlyImages) == 0 {
		return []byte{}, "", errors.New("no images found")
	}

	rand.Seed(time.Now().UnixNano())
	fileName := onlyImages[rand.Intn(len(onlyImages))]

	gif, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", "assets/dancers", fileName.Name()))
	if err != nil {
		log.Error().Err(err).Msg("")
		return []byte{}, "", err
	}

	return gif, fileName.Name(), nil
}

func (params RequiredParams) MRKSHI(mrkshi_phrases *[]string) *[]tgbotapi.MessageConfig {
		var messages []tgbotapi.MessageConfig

		message := params.Update.Message

		msg := tgbotapi.NewMessage(message.Chat.ID, "")

		if random() {
			msg.Text = top_phrase
		} else {
			msg.Text = (*mrkshi_phrases)[rand.Intn(len(*mrkshi_phrases))]
		}

		go analytics.SendToInflux(message.From.String(), message.From.ID, message.Chat.ID, message.Chat.Title, "message", "mrkshi")

		messages = append(messages, msg)

		return &messages
}

func random() bool{
	a := []int{1,1,1,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2} // 3/20 = 15% odd
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	return a[0] & 1 == 1
}
