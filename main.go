package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/go-resty/resty/v2"
	"github.com/ledongthuc/pdf"
)

type Response struct {
	Text    string `json:"text"`
	Summary string `json:"summary"`
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Save the uploaded file
	file, err := os.Create("/tmp/uploaded.pdf")
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to create file."}, err
	}
	defer file.Close()

	_, err = file.Write([]byte(req.Body))
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to write to file."}, err
	}

	// Extract text from the PDF
	f, r, err := pdf.Open(file.Name())
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to open PDF."}, err
	}
	defer f.Close()

	var buf bytes.Buffer
	reader, err := r.GetPlainText()
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to extract text from PDF."}, err
	}

	_, err = io.Copy(&buf, reader)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to read text from PDF."}, err
	}

	text := buf.String()

	// Store text in DynamoDB
	paperID := "some-unique-id"
	err = storeTextInDynamoDB(paperID, text)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to store text in DynamoDB."}, err
	}

	// Generate summary using OpenAI GPT-4
	summary, err := generateSummary(text)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to generate summary."}, err
	}

	// Create response
	response := Response{
		Text:    text,
		Summary: summary,
	}
	responseBody, err := json.Marshal(response)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Failed to marshal response."}, err
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(responseBody),
	}, nil
}

func storeTextInDynamoDB(paperID, text string) error {
	sess := session.Must(session.NewSession())
	svc := dynamodb.New(sess)

	input := &dynamodb.PutItemInput{
		TableName: aws.String("ResearchPapers"),
		Item: map[string]*dynamodb.AttributeValue{
			"PaperID": {
				S: aws.String(paperID),
			},
			"Text": {
				S: aws.String(text),
			},
		},
	}

	_, err := svc.PutItem(input)
	return err
}

func generateSummary(text string) (string, error) {
	client := resty.New()
	response, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer YOUR_OPENAI_API_KEY").
		SetBody(map[string]interface{}{
			"model":      "gpt-4",
			"prompt":     "Summarize the following text: " + text,
			"max_tokens": 100,
		}).
		Post("https://api.openai.com/v1/engines/davinci-codex/completions")

	if err != nil {
		return "", err
	}

	var result map[string]interface{}
	json.Unmarshal(response.Body(), &result)
	summary := result["choices"].([]interface{})[0].(map[string]interface{})["text"].(string)
	return summary, nil
}

func main() {
	lambda.Start(handler)
}
