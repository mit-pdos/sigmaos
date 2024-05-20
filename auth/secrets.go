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

func NewSecretFromProto(psp *proc.ProcSecretProto) *Secret {
	return &Secret{
		ID:  psp.ID,
		Key: psp.Key,
	}
}

func (s *Secret) String() string {
	return fmt.Sprintf("&{ id:%v key:<<redacted>> }", s.ID)
}

func GetAWSSecrets(profile string) (*proc.ProcSecretProto, error) {
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
	return &proc.ProcSecretProto{
		ID:  cfg.Credentials.AccessKeyID,
		Key: cfg.Credentials.SecretAccessKey,
	}, nil
}

func NewAWSCredentialsProvider(s *proc.ProcSecretProto) aws.CredentialsProvider {
	return credentials.NewStaticCredentialsProvider(s.GetID(), s.GetKey(), "")
}
