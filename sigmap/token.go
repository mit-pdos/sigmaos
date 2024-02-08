package sigmap

func NewToken(s Tsigner, signedToken string) *Ttoken {
	return &Ttoken{
		Signer:      string(s),
		SignedToken: signedToken,
	}
}
