package sigmap

func NewPrincipal(id TprincipalID, realm Trealm, token *Ttoken) *Tprincipal {
	return &Tprincipal{
		IDStr:    id.String(),
		RealmStr: realm.String(),
		Token:    token,
	}
}

func (p *Tprincipal) SetID(principalID TprincipalID) {
	p.IDStr = principalID.String()
}

func (p *Tprincipal) GetID() TprincipalID {
	return TprincipalID(p.IDStr)
}

func (p *Tprincipal) GetRealm() Trealm {
	return Trealm(p.RealmStr)
}

func (p *Tprincipal) SetToken(t *Ttoken) {
	p.Token = t
}

func (p *Tprincipal) IsSigned() bool {
	return p.Token != nil && p.Token.GetSignedToken() != NO_SIGNED_TOKEN
}

func (id TprincipalID) String() string {
	return string(id)
}

//func (p *Tprincipal) String() string {
//	return fmt.Sprintf("{ id:%v realm:%v signed:%v }", p.GetID(), p.GetRealm(), p.IsSigned())
//}
