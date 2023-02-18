package main

import (
	"context"
	"strconv"
	"time"
	"encoding/json"
	amqp "github.com/rabbitmq/amqp091-go"
)

type processor struct {
	mq_queue    *amqp.Queue
	mq_channel  *amqp.Channel
	mq_consumer <-chan amqp.Delivery
}

func (proc *processor) sendVoiceToTextReq(voice []byte, chatId int64) {
	corrId := strconv.FormatInt(chatId, 10)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	//
	var err = proc.mq_channel.PublishWithContext(ctx,
		"",          // exchange
		"rpc_queue", // routing key
		false,       // mandatory
		false,       // immediate
		amqp.Publishing{
			ContentType:   "text/plain",
			CorrelationId: corrId,
			ReplyTo:       proc.mq_queue.Name,
			Body:          voice,
		})
	failOnError(err, "Failed to publish a message")
}

func (proc *processor) processReqResult(msg amqp.Delivery) (chatID int64, text string) {
	chatID, err := strconv.ParseInt(msg.CorrelationId, 10, 64)
	if err != nil {
		failOnError(err, "Failed to parse user id")
	}
	var s SendMsg
	err = json.Unmarshal(msg.Body, &s)
	failOnError(err, "Failed to decode msg")
	text = s.Text
	return chatID, text
}
