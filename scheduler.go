package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"io"
	"os"
	"github.com/aws/aws-sdk-go/service/s3"
)

func Handler(event *events.S3Event) error {
	var sqsUrl = os.Getenv("SQS_QUEUE_URL")

	sess := session.New()
	s3Svc := s3.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
	sqsSvc := sqs.New(sess, aws.NewConfig().WithRegion("eu-west-1"))

	for _, record := range event.Records {
		fmt.Println(record.EventName, record.S3.Bucket.Name, record.S3.Object.Key)

		content, err := s3Svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(record.S3.Bucket.Name),
			Key:    aws.String(record.S3.Object.Key),
		})

		if err != nil {
			fmt.Println(err)
			return err
		}

		defer content.Body.Close()
		reader := csv.NewReader(bufio.NewReader(content.Body))

		for {
			line, err := reader.Read()
			if err == io.EOF {
				break
			} else if err != nil {
				fmt.Println(err)
			}
			agreement := &Agreement{
				Id:              line[0],
				OwnerId:         line[1],
				StartDate:       line[2],
				StatementPeriod: line[3],
				Description:     line[4],
				Currency: &Currency{
					Id:     line[5],
					Symbol: line[6],
				},
			}
			sendMessage(agreement, sqsSvc, &sqsUrl)
		}
	}

	return nil
}

func sendMessage(agreement *Agreement, sqsSvc *sqs.SQS, sqsUrl *string) {

	jsonItem, _ := json.Marshal(agreement)

	result, err := sqsSvc.SendMessage(&sqs.SendMessageInput{
		MessageBody: aws.String(string(jsonItem)),
		QueueUrl:    sqsUrl,
	})

	if err != nil {
		fmt.Println(err)
	}

	fmt.Printf("Agreement %s scheduled %s\n", agreement.Id, *result.MessageId)
}

func main() {
	lambda.Start(Handler)
}
