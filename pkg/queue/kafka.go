package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/segmentio/kafka-go"
)

type KafkaProducer struct {
	writer *kafka.Writer
}

type KafkaConsumer struct {
	reader *kafka.Reader
}

func NewKafkaProducer(brokers []string, topic string) *KafkaProducer {
	writer := &kafka.Writer{
		Addr:     kafka.TCP(brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
		Async:    false,
	}

	return &KafkaProducer{writer: writer}
}

func NewKafkaConsumer(brokers []string, topic, groupID string) *KafkaConsumer {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        brokers,
		Topic:          topic,
		GroupID:        groupID,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 1 * time.Second,
		StartOffset:    kafka.FirstOffset,
	})

	return &KafkaConsumer{reader: reader}
}

func (p *KafkaProducer) Publish(ctx context.Context, key string, value interface{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	message := kafka.Message{
		Key:   []byte(key),
		Value: data,
		Time:  time.Now(),
	}

	return p.writer.WriteMessages(ctx, message)
}

func (p *KafkaProducer) PublishBatch(ctx context.Context, messages []Message) error {
	kafkaMessages := make([]kafka.Message, len(messages))
	for i, msg := range messages {
		data, err := json.Marshal(msg.Value)
		if err != nil {
			return fmt.Errorf("failed to marshal message %d: %w", i, err)
		}
		kafkaMessages[i] = kafka.Message{
			Key:   []byte(msg.Key),
			Value: data,
			Time:  time.Now(),
		}
	}
	return p.writer.WriteMessages(ctx, kafkaMessages...)
}

func (c *KafkaConsumer) Subscribe(ctx context.Context, handler func(Message) error) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			message, err := c.reader.ReadMessage(ctx)
			if err != nil {
				return fmt.Errorf("failed to read message: %w", err)
			}

			var value interface{}
			if err := json.Unmarshal(message.Value, &value); err != nil {
				fmt.Printf("Failed to unmarshal message: %v\n", err)
				continue
			}

			msg := Message{
				Key:   string(message.Key),
				Value: value,
				Topic: message.Topic,
			}

			if err := handler(msg); err != nil {
				fmt.Printf("Failed to handle message: %v\n", err)
				continue
			}
		}
	}
}

func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}

func (p *KafkaProducer) Close() error {
	return p.writer.Close()
}

type Message struct {
	Key   string
	Value interface{}
	Topic string
}

type EventType string

const (
	EventUserCreated      EventType = "user_created"
	EventUserUpdated      EventType = "user_updated"
	EventPostCreated      EventType = "post_created"
	EventPostDeleted      EventType = "post_deleted"
	EventFollowCreated    EventType = "follow_created"
	EventFollowDeleted    EventType = "follow_deleted"
	EventLikeCreated      EventType = "like_created"
	EventLikeDeleted      EventType = "like_deleted"
	EventCommentCreated   EventType = "comment_created"
)

type Event struct {
	Type      EventType   `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

type PostEventData struct {
	PostID     string `json:"post_id"`
	UserID     string `json:"user_id"`
	Content    string `json:"content"`
	CreatedAt  string `json:"created_at"`
}

type FollowEventData struct {
	FollowerID  string `json:"follower_id"`
	FollowingID string `json:"following_id"`
	CreatedAt   string `json:"created_at"`
}

type LikeEventData struct {
	UserID string `json:"user_id"`
	PostID string `json:"post_id"`
}

type CommentEventData struct {
	CommentID string `json:"comment_id"`
	UserID    string `json:"user_id"`
	PostID    string `json:"post_id"`
	Content   string `json:"content"`
}