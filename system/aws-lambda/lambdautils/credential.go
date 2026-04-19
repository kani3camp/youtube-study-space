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
	//nolint:staticcheck // Credential JSON is fetched from our managed DynamoDB secret and is limited to the Firebase service account payload.
	return option.WithCredentialsJSON(credentialBytes), nil
}
