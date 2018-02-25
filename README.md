SREome numbers after having spent some time this weekend hacking with s3, lambda, sqs and dynamodb. My main goal was to learn more about this tools in the context of `statement-api` and hopefuly get some inspiration for solving our ETL challenge

I created an experiment with 2 components:

- *Scheduler*: A go lambda function that detects when a new `.csv` file is uploaded to a S3 bucket, reads it and pushes messages (one per csv record) into an SQS queue
- *Loader*: A go lambda function that reads from the SQS queue and inserts the corresponding record in a dynamodb table

As the experiment inputs I used:

- A fixed set of *23300* records (same order of magnitude we expect for statements)
- A *variable number of files* (1 to 4) in which those records are split (more files = more lambdas processing at the same time)
- A *variable write capacity* throughput for the dynamodb table (20 to 160)
- A *variable number of queue consumers* (1 to 4)

*SCHEDULING RESULTS*
- All records in *1* file (2.4MB): Lambda took *153s* to queue all the jobs
- Records split in *2* files of equal size (1.2MB): Lambdas took *77s* to queue all the jobs
- Records split in *4* files of equal size (600KB): Lambdas took *39s* to queue all the jobs

> Main bottleneck were the sequential http requests to SQS. Parallelizing the task in multiple files/lambdas really helped. Batch SQS sends were no attempted so there is definitely margin for optimization

*LOADING RESULTS*
- Write capacity *20*, *1* loader: Lambda *timed out* having processed about 1/3 of the jobs. This was expected given the setting. The goal of this run was to test DynamoDB burst capacity (which allows short bursts above the provisioned capacity based on credits) and to experience throttled requests. Table capacity consumption scaled up to 70 units for a short period of time but quickly went down and lambda started to get throttled requests

- Write capacity *80*, *1* loader: Lambda completed the job after *176s*. Table capacity bursted up to 135 units. No throttled requests were experienced

- Write capacity *160*, *1* loader: Lambda completed the job after *168s*. Table capacity bursted up to 140 units. No throttled requests were experienced
- Write capacity *160*, *2* loaders: Lambdas completed the job after *86s*. Table capacity bursted up to 265 units. No throttled requests were experienced
- Write capacity *160*, *4* loaders: Lambdas completed the job after *44s*. Table capacity bursted up to 328 units. No throttled requests were experienced

> Write capacity setting was key but, after a point, lambda parallelization was needed to exploit it

*CONCLUSION*
... wow, this has been freaking fun :rolling_on_the_floor_laughing: Now we have some real dynamodb numbers and they suggest we can monthly import all our statements in less than 1 minute :wine_glass:
