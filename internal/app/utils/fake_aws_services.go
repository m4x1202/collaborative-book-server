package utils

/*import (
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi"
	"github.com/aws/aws-sdk-go-v2/service/apigatewaymanagementapi/apigatewaymanagementapiiface"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/dynamodbiface"
)

// A fakeDynamoDB instance
type FakeDynamoDB struct {
	dynamodbiface.DynamoDBAPI
	Payload interface{}
	Err     error
}

func (fd *FakeDynamoDB) UpdateItem(input *dynamodb.UpdateItemInput) (*dynamodb.UpdateItemOutput, error) {
	output := new(dynamodb.UpdateItemOutput)
	return output, fd.Err
}

func (fd *FakeDynamoDB) DeleteItem(input *dynamodb.DeleteItemInput) (*dynamodb.DeleteItemOutput, error) {
	output := new(dynamodb.DeleteItemOutput)
	return output, fd.Err
}

func (fd *FakeDynamoDB) Scan(input *dynamodb.ScanInput) (*dynamodb.ScanOutput, error) {
	output := new(dynamodb.ScanOutput)
	output.Items = fd.Payload.([]map[string]*dynamodb.AttributeValue)
	return output, fd.Err
}

func (fd *FakeDynamoDB) Query(input *dynamodb.QueryInput) (*dynamodb.QueryOutput, error) {
	output := new(dynamodb.QueryOutput)
	output.Items = fd.Payload.([]map[string]*dynamodb.AttributeValue)
	return output, fd.Err
}

// A fakeDynamoDB instance
type FakeApiGatewayManagementApi struct {
	apigatewaymanagementapiiface.ApiGatewayManagementApiAPI
	Err error
}

func (fd *FakeApiGatewayManagementApi) PostToConnection(input *apigatewaymanagementapi.PostToConnectionInput) (*apigatewaymanagementapi.PostToConnectionOutput, error) {
	output := new(apigatewaymanagementapi.PostToConnectionOutput)
	return output, fd.Err
}
*/
