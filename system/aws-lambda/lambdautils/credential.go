package lambdautils

import (
	"app.modules/aws-lambda/mydynamodb"
	"google.golang.org/api/option"
)

func FirestoreClientOption() (option.ClientOption, error) {
	credentialBytes, err := mydynamodb.RetrieveFirebaseCredentialInBytes()
	if err != nil {
		return nil, err
	}
	return option.WithCredentialsJSON(credentialBytes), nil
}
