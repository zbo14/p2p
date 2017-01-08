package p2p

// Leave network

type LeaveNetworkArgs struct {
	*Contact
}

func NewLeaveNetworkArgs(id Id, name, peerAddr, routerAddr string) *LeaveNetworkArgs {
	return &LeaveNetworkArgs{
		Contact: NewContact(id, name, peerAddr, routerAddr),
	}
}

type LeaveNetworkReply struct {
	*Contact
}

// Join network // Copy routing table

type CopyTableArgs struct {
	*Contact
}

func NewCopyTableArgs(id Id, name, peerAddr, routerAddr string) *CopyTableArgs {
	return &CopyTableArgs{
		Contact: NewContact(id, name, peerAddr, routerAddr),
	}
}

type CopyTableReply struct {
	*Contact
	Table [ID_SIZE]*list
}

// Find contact

type FindContactArgs struct {
	*Contact
	Target Id
}

func NewFindContactArgs(id Id, name, peerAddr, routerAddr string, target Id) *FindContactArgs {
	return &FindContactArgs{
		Contact: NewContact(id, name, peerAddr, routerAddr),
		Target:  target,
	}
}

type FindContactReply struct {
	*Contact
	Closest Contacts
}

// Ping pong

type Ping struct {
	*Contact
}

func NewPing(id Id, name, peerAddr, routerAddr string) *Ping {
	return &Ping{
		Contact: NewContact(id, name, peerAddr, routerAddr),
	}
}

type Pong struct {
	*Contact
}
