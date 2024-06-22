package lambdautils

import (
	"app.modules/aws-lambda/mydynamodb"
	"fmt"
	"google.golang.org/api/option"
)

// FirestoreClientOption retrieves Firebase credentials from DynamoDB and
// returns a client option suitable for creating a Firestore client.
func FirestoreClientOption() (option.ClientOption, error) {
	credentialBytes, err := mydynamodb.FetchFirebaseCredentialsAsBytes()
	if err != nil {
		return nil, fmt.Errorf("in FetchFirebaseCredentialsAsBytes: %w", err)
	}
	return option.WithCredentialsJSON(credentialBytes), nil
}
