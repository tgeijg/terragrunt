// Package providers defines the interface for a provider.
package providers

import (
	"context"
)

const (
	AWSCredentials CredentialsName = "AWS"
)

type CredentialsName string

type Credentials struct {
	Envs map[string]string
	Name CredentialsName
}

type Provider interface {
	// Name returns the name of the provider.
	Name() string
	// GetCredentials returns a set of credentials.
	GetCredentials(ctx context.Context) (*Credentials, error)
}
