build:
	dep ensure
	env GOOS=linux go build -ldflags="-s -w" -o bin/scheduler scheduler.go types.go
	env GOOS=linux go build -ldflags="-s -w" -o bin/consumer consumer.go types.go
