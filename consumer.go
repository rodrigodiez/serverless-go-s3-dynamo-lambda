package main

import (
	"os"
	"fmt"
	"strconv"
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

type Consumer struct {
	svc *sqs.SQS
	url *string
	jobs chan *sqs.Message
	wg *sync.WaitGroup
}

type Deleter struct {
	svc *sqs.SQS
	url *string
	jobs chan *sqs.Message
	wg *sync.WaitGroup
}

type Inserter struct {
	svc *dynamodb.DynamoDB
	jobs chan *sqs.Message
	deleteJobs chan *sqs.Message
	wg *sync.WaitGroup
}

func NewDeleter(svc *sqs.SQS, url *string, jobs chan *sqs.Message, wg *sync.WaitGroup) *Deleter {
	return &Deleter{
		svc: svc,
		url: url,
		jobs: jobs,
		wg: wg,
	}
}

func NewConsumer(svc *sqs.SQS, url *string, jobs chan *sqs.Message, wg *sync.WaitGroup) *Consumer {
	return &Consumer{
		svc: svc,
		url: url,
		jobs: jobs,
		wg: wg,
	}
}

func NewInserter(svc *dynamodb.DynamoDB, jobs chan *sqs.Message, deleteJobs chan *sqs.Message, wg *sync.WaitGroup) *Inserter {
	return &Inserter{
		svc: svc,
		jobs: jobs,
		deleteJobs: deleteJobs,
		wg: wg,
	}
}

func (w Deleter) Start() {
	w.wg.Add(1)
	fmt.Println("deleter: Added to the wg")

	go func (){
		for job := range w.jobs {
			_, err := w.svc.DeleteMessage(&sqs.DeleteMessageInput{
				QueueUrl:      w.url,
				ReceiptHandle: job.ReceiptHandle,
			})

			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("deleter: message deleted", job.MessageId)
			}
		}

		fmt.Println("deleter: jobs channel closed. I am done")
		w.wg.Done()

		return
	}()
}

func (w Consumer) Start() {
	w.wg.Add(1)
	fmt.Println("consumer: Added to the wg")

	go func (){

		for {
			result, err := w.svc.ReceiveMessage(&sqs.ReceiveMessageInput{
				QueueUrl:            w.url,
				MaxNumberOfMessages: aws.Int64(10),
				VisibilityTimeout:   aws.Int64(60),
				WaitTimeSeconds:     aws.Int64(0),
			})

			if err != nil {
				fmt.Println(err)
				break
			}

			if len(result.Messages) == 0 {
				fmt.Println("consumer: No more messages. I am done")
				w.wg.Done()

				return
			}

			for _, message := range result.Messages {
				w.jobs <- message
			}
		}
	}()

}

func (w Inserter) Start() {
	w.wg.Add(1)
	fmt.Println("inserter: Added to the wg")

	go func () {

		for job := range w.jobs {
			var agreement Agreement

			json.Unmarshal([]byte(*job.Body), &agreement)
			dynamoAgreement, _ := dynamodbattribute.MarshalMap(agreement)

			_, err := w.svc.PutItem(&dynamodb.PutItemInput{
				TableName: aws.String("agreement"),
				Item:      dynamoAgreement,
			})

			if err != nil {
				fmt.Println("Error calling PutItem")
				fmt.Println(err.Error())
			} else {
				w.deleteJobs <- job
			}
		}

		fmt.Println("inserter: jobs channel closed. I am done")
		w.wg.Done()

		return
	}()

}

func Handler(event *events.S3Event) error {
	sqsUrl := os.Getenv("SQS_QUEUE_URL")
	sess := session.New()
 	dynamoSvc := dynamodb.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
 	sqsSvc := sqs.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
	numConsumers, _ := strconv.Atoi(os.Getenv("NUM_CONSUMERS"))
	numInserters, _ := strconv.Atoi(os.Getenv("NUM_INSERTERS"))
	numDeleters, _ := strconv.Atoi(os.Getenv("NUM_DELETERS"))

	jobs := make(chan *sqs.Message, 100)
	deleteJobs := make(chan *sqs.Message, 100)
	consumerWg := new(sync.WaitGroup)
	inserterWg := new(sync.WaitGroup)
	deleterWg := new(sync.WaitGroup)

	for i:=0; i < numConsumers; i++ {
		NewConsumer(sqsSvc, &sqsUrl, jobs, consumerWg).Start()
	}

	for i:=0; i < numInserters; i++ {
		NewInserter(dynamoSvc, jobs, deleteJobs, inserterWg).Start()
	}

	for i:=0; i < numDeleters; i++ {
		NewDeleter(sqsSvc, &sqsUrl, deleteJobs, deleterWg).Start()
	}

	consumerWg.Wait()
	fmt.Println("All consumers finished. Closing jobs channel")
	close(jobs)

	inserterWg.Wait()
	fmt.Println("All inserters finished. Closing deleteJobs channel")
	close(deleteJobs)

	deleterWg.Wait()
	fmt.Println("All deleters finished. We are DONE!")

	return nil
}

func main() {
	lambda.Start(Handler)
}
