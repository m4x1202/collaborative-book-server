package apigateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
)

type mockApiGatewayManagementApiAPI func(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error)

func (m mockApiGatewayManagementApiAPI) PostToConnection(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
	return m(ctx, params, optFns...)
}

func getMockClient(t *testing.T) ApiGatewayManagementApiAPI {
	return mockApiGatewayManagementApiAPI(func(ctx context.Context, params *apigatewaymanagementapi.PostToConnectionInput, optFns ...func(*apigatewaymanagementapi.Options)) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
		t.Helper()
		if params.ConnectionId == nil {
			t.Fatal("expect connectionId to not be nil")
		}
		if e, a := "a", *params.ConnectionId; e != a {
			t.Errorf("expect %v, got %v", e, a)
		}
		if params.Data == nil {
			t.Fatal("expect data to not be nil")
		}
		unmarshaled := &struct {
			A int
		}{}
		if err := json.Unmarshal(params.Data, unmarshaled); err != nil {
			t.Error(err)
		}
		if unmarshaled.A != 1 {
			t.Errorf("expect %v, got %v", 1, unmarshaled.A)
		}
		return &apigatewaymanagementapi.PostToConnectionOutput{}, nil
	})
}

func Test_PostToConnection(t *testing.T) {
	service := WSService{
		client: getMockClient(t),
	}
	if err := service.PostToConnection("a", struct{ A int }{1}); err != nil {
		t.FailNow()
	}
}

func Test_PostToConnections(t *testing.T) {
	service := WSService{
		client: getMockClient(t),
	}
	if err := service.PostToConnections([]string{"a"}, struct{ A int }{1}); err != nil {
		t.FailNow()
	}
}
