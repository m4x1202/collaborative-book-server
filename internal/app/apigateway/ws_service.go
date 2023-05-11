package apigateway

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	cb "github.com/m4x1202/collaborative-book"
)

const (
	APIGatewayEndpoint = "r8sc9tucc2.execute-api.eu-central-1.amazonaws.com/dev"
)

// Ensure WSService implements cb.WSService
var _ cb.WSService = (*WSService)(nil)

// A service that holds apigatewaymanagementapi service functionality
type WSService struct {
	client ApiGatewayManagementApiAPI
}

type ApiGatewayManagementApiAPI interface {
	PostToConnection(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error)
}

func NewWSService(conf aws.Config) WSService {
	return WSService{
		client: apigatewaymanagementapi.NewFromConfig(conf, apigatewaymanagementapi.WithEndpointResolver(apigatewaymanagementapi.EndpointResolverFromURL(APIGatewayEndpoint))),
	}
}

func (wss WSService) PostToConnections(connectionIDs []string, data interface{}) error {
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	var allErrors []error
	for _, connectionID := range connectionIDs {
		if err := wss.postToConnection(connectionID, marshalled); err != nil {
			allErrors = append(allErrors, err)
		}
	}
	if len(allErrors) > 0 {
		return fmt.Errorf("%v", allErrors)
	}
	return nil
}

func (wss WSService) PostToConnection(connectionID string, data interface{}) error {
	marshalled, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return wss.postToConnection(connectionID, marshalled)
}

func (wss WSService) postToConnection(connectionID string, data []byte) error {
	input := &apigatewaymanagementapi.PostToConnectionInput{
		ConnectionId: aws.String(connectionID),
		Data:         data,
	}

	if _, err := wss.client.PostToConnection(context.TODO(), input); err != nil {
		return err
	}
	return nil
}
