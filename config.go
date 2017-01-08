package p2p

func DefaultUser(app App, dir, name string) (*User, error) {
	dht, err := NewDHTRouter("", ROUTER_PORT)
	if err != nil {
		return nil, err
	}
	user := NewUser(app, dir, name)
	_, err = NewPeer(dht, "", PEER_PORT, user)
	if err != nil {
		return nil, err
	}
	return user, nil
}
