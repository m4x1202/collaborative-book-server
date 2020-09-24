package apigateway

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/apigatewaymanagementapi"
	cb "github.com/m4x1202/collaborative-book"
)

const (
	APIGatewayEndpoint = "r8sc9tucc2.execute-api.eu-central-1.amazonaws.com/dev"
)

//Ensure DBService implements cb.DBService
var _ cb.WSService = (*WSService)(nil)

// A service that holds dynamodb db service functionality
type WSService struct {
	apigateway *apigatewaymanagementapi.ApiGatewayManagementApi
}

func NewWSService(sess *session.Session) WSService {
	return WSService{
		apigateway: apigatewaymanagementapi.New(sess, aws.NewConfig().WithEndpoint(APIGatewayEndpoint)),
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

	if _, err := wss.apigateway.PostToConnection(input); err != nil {
		return err
	}
	return nil
}
