package sigmap

func NewPrincipal(id TprincipalID, realm Trealm) *Tprincipal {
	return &Tprincipal{
		IDStr:    id.String(),
		RealmStr: realm.String(),
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

func (id TprincipalID) String() string {
	return string(id)
}
