package auth

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	db "sigmaos/debug"
	"sigmaos/proc"
)

type Secret struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

func NewSecret(id, key string) *Secret {
	return &Secret{
		ID:  id,
		Key: key,
	}
}

func (s *Secret) String() string {
	return fmt.Sprintf("&{ id:%v key:redacted }", s.ID)
}

func GetAWSSecrets() (*proc.ProcSecretProto, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithSharedConfigProfile("sigmaos"))
	if err != nil {
		db.DPrintf(db.ERROR, "Load AWS config: %v", err)
		return nil, err
	}
	creds, err := cfg.Credentials.Retrieve(context.TODO())
	if err != nil {
		db.DPrintf(db.ERROR, "Retreive AWS cred: %v", err)
		return nil, err
	}
	return &proc.ProcSecretProto{
		ID:  creds.AccessKeyID,
		Key: creds.SecretAccessKey,
	}, nil
}

func NewAWSCredentialsProvider(s *proc.ProcSecretProto) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(s.ID, s.Key, "")
}
