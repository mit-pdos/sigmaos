package auth

import (
//	"fmt"

//	"github.com/golang-jwt/jwt"

// db "sigmaos/debug"
)

type AuthClnt struct {
	// token *jwt.Token
}

// XXX right alg to use?
func NewAuthClnt() (*AuthClnt, error) {
	return &AuthClnt{}, nil
	// // Parse the jwt, passing in a function to look up the key.
	// //
	// // Taken from: https://pkg.go.dev/github.com/golang-jwt/jwt#example-Parse-Hmac
	//
	//	token, err := jwt.Parse(tstr, func(token *jwt.Token) (interface{}, error) {
	//		// Validate the alg is expected
	//		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
	//			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
	//		}
	//		// hmacSecret is a []byte containing your secret, e.g. []byte("my_secret_key")
	//		return hmacSecret, nil
	//	})
	//
	//	if err != nil {
	//		db.DPrintf(db.ERROR, "Error parsing jwt: %v", err)
	//		return nil, err
	//	}
	//
	// // TODO: do something with the claims.
	// //  if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
	// //		fmt.Println(claims["foo"], claims["nbf"])
	// //	} else {
	// //		fmt.Println(err)
	// //	}
	//
	//	return &AuthClnt{
	//		token: token,
	//	}, nil
}
