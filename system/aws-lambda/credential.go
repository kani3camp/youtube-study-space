package main

import (
	"google.golang.org/api/option"
)

func FirestoreCredential(credentialBytes []byte) option.ClientOption {
	return option.WithCredentialsJSON(credentialBytes)
}
