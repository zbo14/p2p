package p2p

type Contact struct {
	Id
	Dist       Id
	Name       string
	PeerAddr   string
	RouterAddr string
}

func NewContact(id Id, name, peerAddr, routerAddr string) *Contact {
	return &Contact{
		Id:         id,
		Name:       name,
		PeerAddr:   peerAddr,
		RouterAddr: routerAddr,
	}
}

func NewContactDist(id, dist Id, name, peerAddr, routerAddr string) *Contact {
	return &Contact{id, dist, name, peerAddr, routerAddr}
}

func (ct *Contact) Less(other *Contact) bool {
	return ct.Dist.Less(other.Dist)
}

type Contacts []*Contact

func (cts Contacts) Len() int           { return len(cts) }
func (cts Contacts) Less(i, j int) bool { return cts[i].Less(cts[j]) }
func (cts Contacts) Swap(i, j int)      { cts[i], cts[j] = cts[j], cts[i] }
