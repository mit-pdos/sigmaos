package sigmap

func NewPrincipal(id TprincipalID, token *Ttoken) *Tprincipal {
	return &Tprincipal{
		IDStr: id.String(),
		Token: token,
	}
}

func (p *Tprincipal) SetID(principalID TprincipalID) {
	p.IDStr = principalID.String()
}

func (p *Tprincipal) GetID() TprincipalID {
	return TprincipalID(p.IDStr)
}

func (p *Tprincipal) GetRealm() Trealm {
	return Trealm(p.IDStr)
}

func (p *Tprincipal) SetToken(t *Ttoken) {
	p.Token = t
}

func (id TprincipalID) String() string {
	return string(id)
}
