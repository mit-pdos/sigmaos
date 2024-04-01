package benchmarks_test

import (
	"encoding/json"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	"github.com/stretchr/testify/assert"

	db "sigmaos/debug"
	"sigmaos/test"
)

func spawnLambda(ts *test.RealmTstate, semPath string) {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	client := lambda.New(sess, &aws.Config{Region: aws.String("us-east-1")})
	mnt, err := ts.GetNamedMount()
	assert.Nil(ts.Ts.T, err, "Err GetNamedMOunt: %v", err)
	request := []string{mnt.Addresses()[0].IPPort(), semPath}

	payload, err := json.Marshal(request)
	if err != nil {
		db.DFatalf("Error marshalling lambda request: %v", err)
	}

	result, err := client.Invoke(&lambda.InvokeInput{FunctionName: aws.String("go-spin"), Payload: payload})
	if err != nil {
		db.DFatalf("Error invoking lambda: %v", err)
	}
	assert.Equal(ts.Ts.T, int(*result.StatusCode), 200, "Status code: %v", result.StatusCode)
	if *result.StatusCode != 200 {
		db.DPrintf(db.ALWAYS, "Bad return status %v, msg %v", result.StatusCode, result.Payload)
	}
}
