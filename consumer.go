package main

import (
	"os"
	"fmt"
	"strconv"
	"time"
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"encoding/json"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/sqs"
	"sync"
)

func Handler(ctx context.Context, event *events.S3Event) error {
	var messageCount int = 0;
	sqsUrl := os.Getenv("SQS_QUEUE_URL")
	maxMessages, _ := strconv.ParseInt(os.Getenv("MAX_NUMBER_OF_MESSAGES"), 10, 64)
	visibilityTimeout, _ := strconv.ParseInt(os.Getenv("VISIBILITY_TIMEOUT"), 10, 64)

	deadline, _ := ctx.Deadline()

	sess := session.New()
 	dynamoSvc := dynamodb.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
 	sqsSvc := sqs.New(sess, aws.NewConfig().WithRegion("eu-west-1"))

	for {
		if time.Until(deadline) < 1 * time.Second {
			fmt.Println("Terminating lambda before hard timeout");
			fmt.Println("Processed messages", messageCount);
			return nil
		}

		fmt.Println("Time until deadline", time.Until(deadline))
		result, err := sqsSvc.ReceiveMessage(&sqs.ReceiveMessageInput{
			QueueUrl:            &sqsUrl,
			MaxNumberOfMessages: aws.Int64(maxMessages),
			VisibilityTimeout:   aws.Int64(visibilityTimeout),
			WaitTimeSeconds:     aws.Int64(0),
		})

		if err != nil {
			fmt.Println(err)
			return err
		}

		if len(result.Messages) == 0 {
			fmt.Println("No more messages")
			fmt.Println("Processed messages", messageCount);

			return nil
		}

		var wg sync.WaitGroup

		for _, message := range result.Messages {
			wg.Add(1)
			messageCount ++;
			go handleMessage(message, sqsSvc, dynamoSvc, &wg)
		}

		wg.Wait()
	}
}

func handleMessage(message *sqs.Message, sqsSvc *sqs.SQS, dynamoSvc *dynamodb.DynamoDB, wg *sync.WaitGroup) {
	defer wg.Done()

	var agreement Agreement
	sqsUrl := os.Getenv("SQS_QUEUE_URL")

	json.Unmarshal([]byte(*message.Body), &agreement)
	dynamoAgreement, _ := dynamodbattribute.MarshalMap(agreement)

	_, err := dynamoSvc.PutItem(&dynamodb.PutItemInput{
		TableName: aws.String(os.Getenv("DYNAMODB_TABLE")),
		Item:      dynamoAgreement,
	})

	if err != nil {
		fmt.Println("Error calling PutItem")
		fmt.Println(err.Error())
		return
	}

	_, err = sqsSvc.DeleteMessage(&sqs.DeleteMessageInput{
		QueueUrl:      &sqsUrl,
		ReceiptHandle: message.ReceiptHandle,
	})

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Message deleted", message.MessageId)
}

func main() {
	lambda.Start(Handler)
}
