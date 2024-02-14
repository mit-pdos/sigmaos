package auth

import (
	"crypto/x509"
	"encoding/base64"
	"errors"

	"github.com/golang-jwt/jwt"

	db "sigmaos/debug"
)

type Key interface {
	KeyI() interface{} // Get key as an interface
	B64() []byte       // Get key encoded in base64
	Marshal() string   // Get key in marshaled base64 format
}

type PublicKey interface {
	Key
	Public()        // Ensure Pub/Priv key can't be substituted for each other
	String() string // Get key as a string for printing purposes
}

type PrivateKey interface {
	Key
	Private()       // Ensure Pub/Priv key can't be substituted for each other
	String() string // Get key as a string for printing purposes
}

func NewPublicKey[M jwt.SigningMethod](signingMethod M, b64 []byte) (PublicKey, error) {
	k, err := newKey[M](signingMethod, b64, false)
	if err != nil {
		return nil, err
	}
	return &publicKey{
		Key: k,
	}, nil
}

func NewPrivateKey[M jwt.SigningMethod](signingMethod M, b64 []byte) (PrivateKey, error) {
	k, err := newKey[M](signingMethod, b64, true)
	if err != nil {
		return nil, err
	}
	return &privateKey{
		Key: k,
	}, nil
}

type key struct {
	i   interface{}
	b64 []byte
}

type publicKey struct {
	Key
}

type privateKey struct {
	Key
}

func newKey[M jwt.SigningMethod](m M, b64 []byte, private bool) (*key, error) {
	var i interface{}
	switch any(m).(type) {
	case *jwt.SigningMethodECDSA:
		b, err := base64.StdEncoding.DecodeString(string(b64))
		if err != nil {
			db.DPrintf(db.ERROR, "Error newKey base64.DecodeString: %v", err)
			return nil, err
		}
		if private {
			i, err = x509.ParseECPrivateKey(b)
		} else {
			i, err = x509.ParsePKIXPublicKey(b)
		}
		if err != nil {
			db.DPrintf(db.ERROR, "Error newKey x509.ParseECPrivateKey: %v", err)
			return nil, err
		}
	case *jwt.SigningMethodHMAC:
		i = b64
	default:
		return nil, errors.New("Unkown signing method type")
	}
	return &key{
		i:   i,
		b64: b64,
	}, nil
}

func (k *key) Marshal() string {
	return string(k.b64)
}

func (k *key) B64() []byte {
	return k.b64
}

func (k *key) KeyI() interface{} {
	return k.i
}

func (pk *publicKey) Public() {}

func (pk *publicKey) String() string {
	return string(pk.Key.B64())
}

func (pk *privateKey) Private() {}

func (sk *privateKey) String() string {
	return "priv-key-redacted"
}
