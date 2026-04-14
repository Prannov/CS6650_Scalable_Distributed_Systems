package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

type SQSQueue struct {
	client   *sqs.Client
	queueURL string
}

func NewSQSQueue(queueURL, region string) (*SQSQueue, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &SQSQueue{
		client:   sqs.NewFromConfig(cfg),
		queueURL: queueURL,
	}, nil
}

// Publish sends a JSON-encoded message to SQS.
func (q *SQSQueue) Publish(ctx context.Context, msg any) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = q.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:    aws.String(q.queueURL),
		MessageBody: aws.String(string(body)),
	})
	return err
}

type Message struct {
	Body          string
	ReceiptHandle string
}

// Receive long-polls for up to 10 messages.
func (q *SQSQueue) Receive(ctx context.Context) ([]Message, error) {
	out, err := q.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            aws.String(q.queueURL),
		MaxNumberOfMessages: 10,
		WaitTimeSeconds:     20, // long poll
		VisibilityTimeout:   60,
	})
	if err != nil {
		return nil, err
	}
	var msgs []Message
	for _, m := range out.Messages {
		msgs = append(msgs, Message{Body: *m.Body, ReceiptHandle: *m.ReceiptHandle})
	}
	return msgs, nil
}

// Delete removes a processed message from the queue.
func (q *SQSQueue) Delete(ctx context.Context, receiptHandle string) error {
	_, err := q.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(q.queueURL),
		ReceiptHandle: aws.String(receiptHandle),
	})
	return err
}
