package sigmap

func NewToken(s Tsigner, signedToken string) *Ttoken {
	return &Ttoken{
		SignerStr:   string(s),
		SignedToken: signedToken,
	}
}

func (t *Ttoken) SetSigner(s Tsigner) {
	t.SignerStr = string(s)
}

func (t *Ttoken) GetSigner() Tsigner {
	return Tsigner(t.SignerStr)
}
