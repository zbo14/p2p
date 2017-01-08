package p2p

// linked list

import (
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"net"
	"net/rpc"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
)

// Linked list

type elem struct {
	*Contact
	next, prev *elem
}

func newElem(ct *Contact) *elem {
	return &elem{Contact: ct}
}

type list struct {
	count, size int
	head, tail  *elem
	sync.RWMutex
}

func newList(size int) *list {
	return &list{size: size}
}

// Push contact

func (l *list) pushContactFront(ct *Contact) error {
	if l.count == l.size {
		return errors.New("List has reached capacity")
	}
	e := newElem(ct)
	if l.head != nil {
		l.head.prev = e
		e.prev, e.next = nil, l.head
	} else {
		l.tail = e
	}
	l.head = e
	l.count++
	return nil
}

// Move contact

func (l *list) moveContactFront(ct *Contact) error {
	var e *elem
	for e = l.head; e != nil; e = e.next {
		if e.Equals(ct.Id) {
			break
		}
	}
	if e == nil {
		return errors.New("Cannot find contact")
	} else if e != l.head {
		if e == l.tail {
			e.prev.next = nil
			l.tail = e.prev
		} else {
			e.prev.next = e.next
			e.next.prev = e.prev
		}
		l.head.prev = e
		e.prev, e.next = nil, l.head
		l.head = e
	}
	return nil
}

// Remove contact

func (l *list) removeContact(ct *Contact) error {
	var e *elem
	for e = l.head; e != nil; e = e.next {
		if e.Equals(ct.Id) {
			break
		}
	}
	if e == nil {
		return errors.New("Cannot find contact")
	}
	l.removeElem(e)
	return nil
}

// Iterate func

func (l *list) iterate(fn func(e *elem)) {

	for e := l.head; e != nil; e = e.next {
		fn(e)
	}
}

// Remove element

func (l *list) removeElem(e *elem) {
	// 'e' shouldn't be nil
	if e == l.head {
		e.next.prev = nil
		l.head = e.next
	} else if e == l.tail {
		e.prev.next = nil
		l.tail = e.prev
	} else {
		e.prev.next = e.next
		e.next.prev = e.prev
	}
	l.count--
}

// DHT routing table

const BUCKET_SIZE = 20
const ROUTER_PORT = 17771

type DHTRouter struct {
	addr     string
	lis      net.Listener
	logger   log15.Logger
	mtx      sync.RWMutex
	nameIds  map[string]Ids
	peer     *Peer
	shutdown uint32
	table    [ID_SIZE]*list
	user     *User
}

func NewDHTRouter(host string, port int) (*DHTRouter, error) {
	if !ValidIP(host) {
		ip, err := ExternalIP()
		if err != nil {
			return nil, err
		}
		host = ip.String()
	}
	if port < 1000 {
		port = ROUTER_PORT
	}
	dht := new(DHTRouter)
	dht.addr = host + ":" + strconv.Itoa(port)
	for i, _ := range dht.table {
		dht.table[i] = newList(BUCKET_SIZE)
	}
	dht.logger = log15.New("module", "dht-router")
	if err := rpc.Register(dht); err != nil {
		return nil, err
	}
	return dht, nil
}

func (dht *DHTRouter) getNameIds(name string) (Ids, error) {
	dht.mtx.RLock()
	defer dht.mtx.RUnlock()
	ids, ok := dht.nameIds[name]
	if !ok {
		return nil, errors.New("Could not find id(s) for name")
	} else if len(ids) == 0 {
		//.. shouldn't happen
	}
	return ids, nil
}

func (dht *DHTRouter) setNameId(id Id, name string) error {
	dht.mtx.Lock()
	defer dht.mtx.Unlock()
	ids := dht.nameIds[name]
	for i, _ := range ids {
		if id.Equals(ids[i]) {
			return errors.New("Already found id for name")
		}
	}
	dht.nameIds[name] = append(ids, id)
	return nil
}

func (dht *DHTRouter) removeNameId(id Id, name string) error {
	dht.mtx.Lock()
	defer dht.mtx.Unlock()
	ids, ok := dht.nameIds[name]
	if !ok {
		return errors.New("Could not find id for name")
	}
	for i, _ := range ids {
		if id.Equals(ids[i]) {
			if len(ids) == 1 {
				delete(dht.nameIds, name)
			} else {
				dht.nameIds[name] = append(ids[:i], ids[i+1:]...)
			}
			return nil
		}
	}
	return errors.New("Could not find id for name")
}

func (dht *DHTRouter) boot() error {
	if err := dht.initListener(); err != nil {
		return err
	}
	return nil
}

func (dht *DHTRouter) start(bootAddr string) error {
	if err := dht.initListener(); err != nil {
		return err
	}
	if err := dht.joinNetwork(bootAddr); err != nil {
		return err
	}
	return nil
}

func (dht *DHTRouter) stop() error {
	shutdown := atomic.CompareAndSwapUint32(&dht.shutdown, 0, 1)
	if !shutdown {
		return errors.New("Could not shut down router")
	}
	go dht.leaveNetwork()
	return nil
}

func (dht *DHTRouter) initListener() error {
	if dht.lis == nil {
		lis, err := net.Listen("tcp", dht.addr)
		if err != nil {
			return err
		}
		dht.lis = lis
	}
	return nil
}

func (dht *DHTRouter) acceptConnections() {
	for {
		if atomic.LoadUint32(&dht.shutdown) == 1 {
			return
		}
		conn, err := dht.lis.Accept()
		if err != nil {
			dht.logger.Error(err.Error())
			continue
		}
		go rpc.ServeConn(conn)
	}
}

func (dht *DHTRouter) index(id Id) int {
	for i, b := range dht.peer.id[:] {
		for j := 0; j < 8; j++ {
			if b>>uint8(7-j)^id[i]>>uint8(7-j) > 0 {
				return i*8 + j
			}
		}
	}
	return ID_SIZE
}

func (dht *DHTRouter) xor(id Id) (dist Id) {
	for i := 0; i < ID_LENGTH; i++ {
		dist[i] = dht.peer.id[i] ^ id[i]
	}
	return
}

func (dht *DHTRouter) update(ct *Contact) error {
	idx := dht.index(ct.Id)
	if idx == ID_SIZE {
		return errors.New("Cannot update contact with same Id")
	}
	bucket := dht.table[idx]
	bucket.Lock()
	defer bucket.Unlock()
	if err := bucket.moveContactFront(ct); err != nil {
		if err = bucket.pushContactFront(ct); err != nil {
			bucket.iterate(func(e *elem) {
				if _, err := dht.ping(e.RouterAddr); err != nil {
					bucket.removeElem(e)
				}
			})
			// Try pushing contact again
			if err = bucket.pushContactFront(ct); err != nil {
				return err
			}
		}
	}
	if err := dht.setNameId(ct.Id, ct.Name); err != nil {
		return err
	}
	return nil
}

func (dht *DHTRouter) remove(ct *Contact) error {
	idx := dht.index(ct.Id)
	if idx == ID_SIZE {
		return errors.New("Cannot remove contact with same Id")
	}
	bucket := dht.table[idx]
	bucket.Lock()
	defer bucket.Unlock()
	if err := bucket.removeContact(ct); err != nil {
		return err
	}
	if err := dht.removeNameId(ct.Id, ct.Name); err != nil {
		return err
	}
	return nil
}

func (dht *DHTRouter) call(addr, method string, args, reply interface{}) error {
	cli, err := rpc.Dial("tcp", addr)
	if err != nil {
		return err
	}
	if err = cli.Call(method, args, reply); err != nil {
		return err
	}
	return nil
}

func (dht *DHTRouter) validateContact(ct *Contact) error {
	if !ValidIP(ct.PeerAddr) {
		return errors.New("Invalid Peer Address")
	}
	if !ValidIP(ct.RouterAddr) {
		return errors.New("Invalid Router Address")
	}
	id := NewId([]byte(ct.PeerAddr + ct.RouterAddr))
	if !id.Equals(ct.Id) {
		return errors.New("Invalid Peer Id")
	}
	return nil
}

// join network

func (dht *DHTRouter) joinNetwork(bootAddr string) error {
	// Copy routing table
	if err := dht.copyTable(bootAddr); err != nil {
		return err
	}
	// Ping contacts so they add us to their tables
	for _, bucket := range dht.table {
		bucket.RLock()
		bucket.iterate(func(e *elem) {
			_, err := dht.ping(e.RouterAddr)
			if err != nil {
				//..
			}
		})
		bucket.RUnlock()
	}
	dht.logger.Info("joined network!")
	return nil
}

// Leave network

func (dht *DHTRouter) leaveNetwork() {
	for _, bucket := range dht.table {
		bucket.RLock()
		bucket.iterate(func(e *elem) {
			args := NewLeaveNetworkArgs(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
			reply := new(LeaveNetworkReply)
			dht.call(e.RouterAddr, "DHTRouter.LeaveNetworkHandler", args, reply) //ignore err
			//..
		})
		bucket.RUnlock()
	}
}

func (dht *DHTRouter) LeaveNetworkHandler(args *LeaveNetworkArgs, reply *LeaveNetworkReply) error {
	if err := dht.validateContact(args.Contact); err != nil {
		return err
	}
	if err := dht.remove(args.Contact); err != nil {
		return err
	}
	reply.Contact = NewContact(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
	return nil
}

// Copy routing table

func (dht *DHTRouter) copyTable(bootAddr string) error {
	args := NewCopyTableArgs(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
	reply := new(CopyTableReply)
	err := dht.call(bootAddr, "DHTRouter.CopyTableHandler", args, reply)
	if err != nil {
		return err
	}
	for _, bucket := range reply.Table {
		bucket.iterate(func(e *elem) {
			go func(c *Contact) {
				if err = dht.update(e.Contact); err != nil {
					dht.logger.Warn("Could not add contact to routing table")
				}
			}(e.Contact)
		})
	}
	return nil
}

func (dht *DHTRouter) CopyTableHandler(args *CopyTableArgs, reply *CopyTableReply) error {
	if err := dht.validateContact(args.Contact); err != nil {
		return err
	}
	if err := dht.update(args.Contact); err != nil {
		return err
	}
	reply.Contact = NewContact(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
	reply.Table = dht.table
	return nil
}

// Find Contacts

func (dht *DHTRouter) findContacts(target Id) Contacts {
	replies := make(chan *FindContactReply)
	seen := make(map[Id]struct{})
	querying := make(map[Id]struct{})
	cts := dht.findClosest(target)
	for _, ct := range cts {
		reply, err := dht.findContact(ct.RouterAddr, target)
		if err != nil {
			dht.logger.Error(err.Error())
		} else {
			go func() { replies <- reply }()
		}
		querying[ct.Id] = struct{}{}
		seen[ct.Id] = struct{}{}
	}
	for len(querying) > 0 {
		reply := <-replies
		if _, ok := querying[reply.Id]; !ok {
			dht.logger.Warn("Unexpected FindContactReply")
		}
		delete(querying, reply.Id)
		for _, ct := range reply.Closest {
			if _, ok := seen[ct.Id]; !ok {
				cts = append(cts, ct)
				_reply, err := dht.findContact(ct.RouterAddr, target)
				if err != nil {
					dht.logger.Error(err.Error())
				} else {
					go func() { replies <- _reply }()
				}
				querying[_reply.Id] = struct{}{}
				seen[_reply.Id] = struct{}{}
			}
		}
	}
	sort.Sort(cts)
	if len(cts) > BUCKET_SIZE {
		cts = cts[:BUCKET_SIZE]
	}
	return cts
}

func (dht *DHTRouter) findContact(addr string, target Id) (*FindContactReply, error) {
	args := NewFindContactArgs(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr, target)
	reply := new(FindContactReply)
	err := dht.call(addr, "DHTRouter.FindContactHandler", args, reply)
	if err != nil {
		return nil, err
	}
	return reply, nil
}

func (dht *DHTRouter) FindContactHandler(args *FindContactArgs, reply *FindContactReply) (err error) {
	if err = dht.validateContact(args.Contact); err != nil {
		return err
	}
	if err = dht.update(args.Contact); err != nil {
		return err
	}
	reply.Contact = NewContact(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
	reply.Closest = dht.findClosest(args.Target)
	return nil
}

func (dht *DHTRouter) findClosest(id Id) (cts Contacts) {
	idx := dht.index(id)
	for i := idx; i >= 0 && len(cts) < BUCKET_SIZE; {
		if i == ID_SIZE {
			i = idx - 1
		}
		bucket := dht.table[idx]
		bucket.Lock()
		e := bucket.head
		dist := dht.xor(e.Id)
		ct := NewContactDist(e.Id, dist, e.Name, e.PeerAddr, e.RouterAddr)
		bucket.Unlock()
		cts = append(cts, ct)
		if i >= idx {
			i++
		} else {
			i--
		}
	}
	sort.Sort(cts)
	return
}

// Ping Pong

func (dht *DHTRouter) ping(addr string) (*Pong, error) {
	args := NewPing(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
	reply := new(Pong)
	err := dht.call(addr, "DHTRouter.PingHandler", args, reply)
	if err != nil {
		return nil, err
	}
	if err = dht.validateContact(reply.Contact); err != nil {
		return nil, err
	}
	return reply, nil
}

func (dht *DHTRouter) PingHandler(args *Ping, reply *Pong) (err error) {
	if err = dht.validateContact(args.Contact); err != nil {
		return err
	}
	reply.Contact = NewContact(dht.peer.id, dht.user.name, dht.peer.addr, dht.addr)
	return nil
}
