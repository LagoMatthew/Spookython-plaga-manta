package main

import (
	"context"
	"encoding/json"
	"spookython-backend/internal/domain"
	"spookython-backend/internal/engine"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

func handler(ctx context.Context, request events.APIGatewayV2HTTPRequest) (events.APIGatewayV2HTTPResponse, error) {
	headers := map[string]string{
		"Access-Control-Allow-Origin":  "*",
		"Access-Control-Allow-Methods": "POST, GET, OPTIONS",
		"Access-Control-Allow-Headers": "Content-Type",
		"Content-Type":                 "application/json",
	}

	if request.RequestContext.HTTP.Method == "OPTIONS" {
		return events.APIGatewayV2HTTPResponse{
			StatusCode: 200,
			Headers:    headers,
		}, nil
	}

	var req domain.SimulationRequest
	if request.Body != "" {
		_ = json.Unmarshal([]byte(request.Body), &req)
	}

	response := engine.RunFullSimulation(req)
	bodyBytes, _ := json.Marshal(response)

	return events.APIGatewayV2HTTPResponse{
		StatusCode: 200,
		Headers:    headers,
		Body:       string(bodyBytes),
	}, nil
}

func main() {
	lambda.Start(handler)
}
