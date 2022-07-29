package main

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type (
	Pusher struct {
		producer *kafka.Conn
		topic    string
	}
)

func newPusher(ctx context.Context, cfg KafkaConfig) *Pusher {
	conn, err := kafka.DialLeader(ctx, "tcp", cfg.Brokers[0], cfg.Topic, cfg.Partition)
	if err != nil {
		logger.Error(err)
		return nil
	}
	pusher := &Pusher{
		producer: conn,
		topic:    cfg.Topic,
	}
	return pusher
}

func (p *Pusher) Close() error {
	if p.producer != nil {
		return p.producer.Close()
	}
	return nil
}

func (p *Pusher) Name() string {
	return p.topic
}

func (p *Pusher) Push(v string) error {
	msg := kafka.Message{
		Value: []byte(v),
	}
	_, err := p.producer.WriteMessages(msg)
	return err
}
