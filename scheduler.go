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

type Publisher struct {
	svc *sqs.SQS
	url *string
	jobs chan []string
	wg *sync.WaitGroup
}

type Scanner struct {
	svc *s3.S3
	bucket *string
	key *string
	jobs chan []string
	wg *sync.WaitGroup
}

func NewPublisher(svc *sqs.SQS, url *string, jobs chan []string, wg *sync.WaitGroup) *Publisher {
	return &Publisher{
		svc: svc,
		url: url,
		jobs: jobs,
		wg: wg,
	}
}

func NewScanner(svc *s3.S3, bucket *string, key *string, jobs chan []string, wg *sync.WaitGroup) *Scanner {
	return &Scanner{
		svc: svc,
		bucket: bucket,
		key: key,
		jobs: jobs,
		wg: wg,
	}
}

func (w Publisher) Start(){
	w.wg.Add(1)
	fmt.Println("publisher: Added to the wg")

	go func(){

		for line := range w.jobs {
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

			result, err := w.svc.SendMessage(&sqs.SendMessageInput{
				MessageBody: aws.String(string(jsonItem)),
				QueueUrl:    w.url,
			})

			if err != nil {
				fmt.Println(err)
			}

			fmt.Printf("Job %s scheduled %s\n", agreement.Id, *result.MessageId)
		}

		fmt.Println("worker: jobs channel closed. I am done")
		w.wg.Done()
	}()
}

func (w Scanner) Start(){
	w.wg.Add(1)
	fmt.Println("scanner: Added to the wg")

	go func () {
		content, err := w.svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(*w.bucket),
			Key:    aws.String(*w.key),
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

			w.jobs <- line
		}

		fmt.Println("All file read. Closing jobs channel")
		close(w.jobs)
		w.wg.Done()
	}()
}

func Handler(event *events.S3Event) error {
	var sqsUrl = os.Getenv("SQS_QUEUE_URL")

	sess := session.New()
	s3Svc := s3.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
	sqsSvc := sqs.New(sess, aws.NewConfig().WithRegion("eu-west-1"))
	jobs := make(chan []string, 100)
	numPublishers, _ := strconv.Atoi(os.Getenv("NUM_PUBLISHERS"))
	wg := new(sync.WaitGroup)

	for _, record := range event.Records {
		fmt.Println(record.EventName, record.S3.Bucket.Name, record.S3.Object.Key)

		NewScanner(s3Svc, &record.S3.Bucket.Name, &record.S3.Object.Key, jobs, wg).Start()

		for i:=0; i < numPublishers; i++ {
			NewPublisher(sqsSvc, &sqsUrl, jobs, wg).Start()
		}

		fmt.Println("main: waiting for workers to finish")
		wg.Wait()
	}

	return nil
}

func main() {
	lambda.Start(Handler)
}
