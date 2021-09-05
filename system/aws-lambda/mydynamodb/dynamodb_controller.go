package mydynamodb

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"log"
	"os"
)





func RetrieveFirebaseCredentialInBytes() ([]byte, error) {
	log.Println("RetrieveFirebaseCredentialInBytes()")
	region := os.Getenv("AWS_REGION")	// Lambda関数と同じregionのDyanamoDBテーブル
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
	
	// データstruct
	type SecretData struct {
		SecretData string `dynamodbav:"secret_data"`
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
