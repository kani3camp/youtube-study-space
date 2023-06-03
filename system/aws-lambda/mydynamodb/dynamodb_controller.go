package mydynamodb

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"log"
	"os"
)

type SecretData struct {
	SecretData string `dynamodbav:"secret_data"`
}

func FetchFirebaseCredentialsAsBytes() ([]byte, error) {
	region := os.Getenv("AWS_REGION") // Use the same region as the Lambda function for the DynamoDB table
	sess := session.Must(session.NewSession())
	db := dynamodb.New(sess, aws.NewConfig().WithRegion(region))
	
	params := &dynamodb.GetItemInput{
		TableName: aws.String("secrets"),
		Key: map[string]*dynamodb.AttributeValue{
			"secret_name": {
				S: aws.String(SecretNameFirestore),
			},
		},
	}
	
	result, err := db.GetItem(params)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	secretData := SecretData{}
	err = dynamodbattribute.UnmarshalMap(result.Item, &secretData)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	return []byte(secretData.SecretData), nil
}
