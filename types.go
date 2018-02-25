package main

type Currency struct {
	Id     string `dynamodbav:"id" json:"id"`
	Symbol string `dynamodbav:"symbol" json:"symbol"`
}

type Agreement struct {
	Id              string    `dynamodbav:"id" json:"id"`
	Description     string    `dynamodbav:"description" json:"description"`
	OwnerId         string    `dynamodbav:"owner_id" json:"owner_id"`
	StartDate       string    `dynamodbav:"start_date" json:"start_date"`
	StatementPeriod string    `dynamodbav:"statement_period" json:"statement_period"`
	Currency        *Currency `dynamodbav:"currency" json:"currency"`
}
