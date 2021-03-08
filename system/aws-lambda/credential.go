package main

import (
	"google.golang.org/api/option"
)

func FirestoreClientOption(credentialBytes []byte) option.ClientOption {
	return option.WithCredentialsJSON(credentialBytes)
}
