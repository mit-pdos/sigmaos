package auth

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"

	db "sigmaos/debug"
	sp "sigmaos/sigmap"
)

type Secret struct {
	ID       string `json:"id"`
	Key      string `json:"key"`
	Metadata string `json:"metadata"`
}

func NewSecret(id, key, metadata string) *Secret {
	return &Secret{
		ID:       id,
		Key:      key,
		Metadata: metadata,
	}
}

func NewSecretFromProto(psp *sp.SecretProto) *Secret {
	return &Secret{
		ID:       psp.ID,
		Key:      psp.Key,
		Metadata: psp.Metadata,
	}
}

func (s *Secret) String() string {
	return fmt.Sprintf("&{ id:%v key:<<redacted>> }", s.ID)
}

func GetAWSSecrets(profile string) (*sp.SecretProto, error) {
	sharedCredsFiles := []string{
		config.DefaultSharedCredentialsFilename(),
		"/home/sigmaos/.aws/credentials",
	}
	sharedConfFiles := []string{
		config.DefaultSharedConfigFilename(),
		"/home/sigmaos/.aws/config",
	}
	cfg, err := config.LoadSharedConfigProfile(
		context.TODO(),
		profile,
		func(o *config.LoadSharedConfigOptions) {
			o.ConfigFiles = sharedConfFiles
			o.CredentialsFiles = sharedCredsFiles
		},
	)
	if err != nil {
		db.DPrintf(db.ERROR, "Load AWS config: %v", err)
		return nil, err
	}
	return &sp.SecretProto{
		ID:       cfg.Credentials.AccessKeyID,
		Key:      cfg.Credentials.SecretAccessKey,
		Metadata: cfg.Region,
	}, nil
}

func NewAWSCredentialsProvider(s *sp.SecretProto) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(s.GetID(), s.GetKey(), "")
}
