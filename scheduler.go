package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sync"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"io"
	"os"
	"github.com/aws/aws-sdk-go/service/s3"
	"strconv"
)

func Handler(event *events.S3Event) error {
	var sqsUrl = os.Getenv("SQS_QUEUE_URL")

	sess := session.New()
	s3Svc := s3.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
	sqsSvc := sqs.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
	jobs := make(chan []string, 100)
	numWorkers, _ := strconv.Atoi(os.Getenv("NUM_WORKERS"))
	wg := new(sync.WaitGroup)

	for _, record := range event.Records {
		fmt.Println(record.EventName, record.S3.Bucket.Name, record.S3.Object.Key)

		wg.Add(1)

		go func (){
			fmt.Println("scanner: Added to the wg")


			content, err := s3Svc.GetObject(&s3.GetObjectInput{
				Bucket: aws.String(record.S3.Bucket.Name),
				Key:    aws.String(record.S3.Object.Key),
			})

			if err != nil {
				fmt.Println(err)
				return
			}

			defer content.Body.Close()
			reader := csv.NewReader(bufio.NewReader(content.Body))

			for {
				line, err := reader.Read()
				if err == io.EOF {
					break
				} else if err != nil {
					fmt.Println(err)
					break
				}

				jobs <- line
			}

			fmt.Println("All file read. Closing jobs channel")
			close(jobs)
			wg.Done()
		}()

		for i:=0; i < numWorkers; i++ {
			wg.Add(1)

			go func (){
				fmt.Println("worker: Added to the wg")

				for line := range jobs {
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

					jsonItem, _ := json.Marshal(agreement)

					result, err := sqsSvc.SendMessage(&sqs.SendMessageInput{
						MessageBody: aws.String(string(jsonItem)),
						QueueUrl:    &sqsUrl,
					})

					if err != nil {
						fmt.Println(err)
					}

					fmt.Printf("Agreement %s scheduled %s\n", agreement.Id, *result.MessageId)
				}

				fmt.Println("worker: jobs channel exhausted. we are done")
				wg.Done()
			}()
		}

		fmt.Println("main: waiting for routines to finish")
		wg.Wait()
	}

	return nil
}

func main() {
	lambda.Start(Handler)
}
