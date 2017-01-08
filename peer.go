package p2p

import (
	"github.com/inconshreveable/log15"
	"github.com/pkg/errors"
	"io"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
)

const PEER_PORT = 7771

type Peer struct {
	addr      string
	conns     chan net.Conn
	dht       *DHTRouter
	finds     map[string]Ids
	findsMtx  sync.Mutex
	id        Id
	lis       net.Listener
	logger    log15.Logger
	shutdown  uint32
	stores    map[string]struct{}
	storesMtx sync.Mutex
	user      *User
}

func NewPeer(dht *DHTRouter, host string, port int, user *User) (*Peer, error) {
	if !ValidIP(host) {
		ip, err := ExternalIP()
		if err != nil {
			return nil, err
		}
		host = ip.String()
	}
	if port < 1000 {
		port = PEER_PORT
	}
	addr := host + ":" + strconv.Itoa(port)
	p := &Peer{
		addr:   addr,
		conns:  make(chan net.Conn),
		finds:  make(map[string]Ids),
		id:     NewId([]byte(addr + dht.addr)),
		logger: log15.New("module", "peer"),
		dht:    dht,
		stores: make(map[string]struct{}),
		user:   user,
	}
	dht.peer, dht.user = p, user
	user.dht, user.peer = dht, p
	return p, nil
}

func (p *Peer) start() error {
	if err := p.initListener(); err != nil {
		return err
	}
	go p.acceptConnections()
	go p.HandleMessages()
	return nil
}

func (p *Peer) stop() error {
	shutdown := atomic.CompareAndSwapUint32(&p.shutdown, 0, 1)
	if !shutdown {
		return errors.New("Could not shut down peer")
	}
	return nil
}

func (p *Peer) initListener() error {
	if p.lis == nil {
		lis, err := net.Listen("tcp", p.addr)
		if err != nil {
			return err
		}
		p.lis = lis
	}
	return nil
}

// must init listener before starting the routine
func (p *Peer) acceptConnections() {
	for {
		if atomic.LoadUint32(&p.shutdown) == 1 {
			return
		}
		conn, err := p.lis.Accept()
		if err != nil {
			p.logger.Error(err.Error())
			continue
		}
		p.conns <- conn
	}
}

func (p *Peer) writeMessage(msg Message) {
	cts := p.dht.findContacts(msg.To())
	for _, ct := range cts {
		conn, err := net.Dial("tcp", ct.PeerAddr)
		if err != nil {
			conn.Close()
			p.logger.Error(err.Error())
			continue
		}
		if err = EncodeMessage(msg, conn); err != nil {
			p.logger.Error(err.Error())
		}
		conn.Close()
	}
}

func (p *Peer) writeMessageAndFiles(msg Message, r io.Reader) {
	cts := p.dht.findContacts(msg.To())
	for _, ct := range cts {
		conn, err := net.Dial("tcp", ct.PeerAddr)
		if err != nil {
			p.logger.Error(err.Error())
			continue
		}
		if err = EncodeMessage(msg, conn); err != nil {
			conn.Close()
			p.logger.Error(err.Error())
			continue
		}
		r = io.TeeReader(r, conn)
		conn.Close()
	}
}

// Requests

func (p *Peer) FindFileRequest(key []byte, to Id) error {
	req := NewFindFileRequest(p.id, key, to)
	p.findsMtx.Lock()
	defer p.findsMtx.Unlock()
	ids, ok := p.finds[Hexstr(key)]
	if ok {
		for _, id := range ids {
			if to.Equals(id) {
				return errors.New("FindFile for key, id already requested")
			}
		}
	}
	p.finds[Hexstr(key)] = append(ids, to)
	go p.writeMessage(req)
	return nil
}

func (p *Peer) StoreFileRequest(filesize int64, key []byte, r io.Reader) error {
	req := NewStoreFileRequest(p.id, filesize, key)
	p.storesMtx.Lock()
	defer p.storesMtx.Unlock()
	if _, ok := p.stores[Hexstr(key)]; ok {
		return errors.New("StoreFile for key already requested")
	}
	p.stores[Hexstr(key)] = struct{}{}
	go p.writeMessageAndFiles(req, r)
	return nil
}

func (p *Peer) FindDirRequest(key []byte, to Id) error {
	req := NewFindDirRequest(p.id, key, to)
	p.findsMtx.Lock()
	defer p.findsMtx.Unlock()
	ids, ok := p.finds[Hexstr(key)]
	if ok {
		for _, id := range ids {
			if to.Equals(id) {
				return errors.New("FindDir for key, id already requested")
			}
		}
	}
	p.finds[Hexstr(key)] = append(ids, to)
	go p.writeMessage(req)
	return nil
}

func (p *Peer) StoreDirRequest(filenames []string, filesizes []int64, key []byte, r io.Reader) error {
	req := NewStoreDirRequest(p.id, filenames, filesizes, key)
	p.storesMtx.Lock()
	defer p.storesMtx.Unlock()
	if _, ok := p.stores[Hexstr(key)]; ok {
		return errors.New("StoreDir for key already requested")
	}
	p.stores[Hexstr(key)] = struct{}{}
	go p.writeMessageAndFiles(req, r)
	return nil
}

// Responses

func (p *Peer) FindFileResponse(err error, filesize int64, key []byte, r io.Reader, target, to Id) {
	if err != nil {
		res := NewFindFileResponse(err, 0, p.id, key, target, to)
		go p.writeMessage(res)
		return
	}
	res := NewFindFileResponse(nil, filesize, p.id, key, target, to)
	go p.writeMessageAndFiles(res, r)
}

func (p *Peer) StoreFileResponse(err error, key []byte, to Id) {
	res := NewStoreFileResponse(err, p.id, key, to)
	go p.writeMessage(res)
}

func (p *Peer) FindDirResponse(err error, filenames []string, filesizes []int64, key []byte, r io.Reader, target, to Id) {
	if err != nil {
		res := NewFindDirResponse(err, nil, nil, p.id, key, target, to)
		go p.writeMessage(res)
		return
	}
	res := NewFindDirResponse(nil, filenames, filesizes, p.id, key, target, to)
	go p.writeMessageAndFiles(res, r)
}

func (p *Peer) StoreDirResponse(err error, key []byte, to Id) {
	res := NewStoreDirResponse(err, p.id, key, to)
	go p.writeMessage(res)
}

func (p *Peer) HandleMessages() {
	for {
		if atomic.LoadUint32(&p.shutdown) == 1 {
			return
		}
		conn, ok := <-p.conns
		if !ok {
			panic("Conns channel closed unexpectedly")
		}
		msg, err := DecodeMessage(conn)
		if err != nil {
			p.logger.Error(err.Error())
			conn.Close()
			continue
		}

		key := msg.Key()
		to := msg.From()

		// Handle message
		switch msg.(type) {
		case *FindFileRequest:
			filesize, r, err := p.user.FindFile(key)
			p.FindFileResponse(err, filesize, key, r, msg.To(), to)
		case *FindFileResponse:
			target := msg.(*FindFileResponse).Target
			p.findsMtx.Lock()
			ids, ok := p.finds[Hexstr(key)]
			if !ok {
				p.findsMtx.Unlock()
				// Either we received an unexpected response
				// or a response for a file we queried and
				// already received.. Ignore both for now
				p.logger.Warn("Ignoring FindFileResponse")
			} else {
				var i int
				for i, _ = range ids {
					if target.Equals(ids[i]) {
						// Should we wait for more responses
						// before deleting the entry?..
						delete(p.finds, Hexstr(key))
						p.findsMtx.Unlock()
						err = msg.(*FindFileResponse).Error
						filesize := msg.(*FindFileResponse).Filesize
						go p.user.HandleFile(err, filesize, key, conn)
						break
					}
				}
				if i == len(ids) {
					p.logger.Warn("Received FindFileResponse for key, unlisted id")
				}
			}
		case *StoreFileRequest:
			filesize := msg.(*StoreFileRequest).Filesize
			err = p.user.StoreFile(filesize, key, conn)
			p.StoreFileResponse(err, key, to)
		case *StoreFileResponse:
			p.storesMtx.Lock()
			if _, ok := p.stores[Hexstr(key)]; !ok {
				p.storesMtx.Unlock()
				p.logger.Warn("Ignoring StoreFileResponse")
			} else if err = msg.(*StoreFileResponse).Error; err == nil {
				delete(p.finds, Hexstr(key))
				p.storesMtx.Unlock()
			} else {
				//..
			}
		case *FindDirRequest:
			filenames, filesizes, r, err := p.user.FindDir(key)
			p.FindDirResponse(err, filenames, filesizes, key, r, msg.To(), to)
		case *FindDirResponse:
			target := msg.(*FindDirResponse).Target
			p.findsMtx.Lock()
			ids, ok := p.finds[Hexstr(key)]
			if !ok {
				p.findsMtx.Unlock()
				p.logger.Warn("Ignoring FindDirResponse")
			} else {
				var i int
				for i, _ = range ids {
					if target.Equals(ids[i]) {
						delete(p.finds, Hexstr(key))
						p.findsMtx.Unlock()
						err = msg.(*FindDirResponse).Error
						filenames := msg.(*FindDirResponse).Filenames
						filesizes := msg.(*FindDirResponse).Filesizes
						go p.user.HandleDir(err, filenames, filesizes, key, conn)
						break
					}
				}
				if i == len(ids) {
					p.logger.Warn("Received FindDirResponse for key, unlisted id")
				}
			}
		case *StoreDirRequest:
			filenames := msg.(*StoreDirRequest).Filenames
			filesizes := msg.(*StoreDirRequest).Filesizes
			err = p.user.StoreDir(filenames, filesizes, key, conn)
			p.StoreDirResponse(err, key, to)
		case *StoreDirResponse:
			p.storesMtx.Lock()
			if _, ok := p.stores[Hexstr(key)]; !ok {
				p.storesMtx.Unlock()
				p.logger.Warn("Ignoring StoreDirResponse")
			} else if err = msg.(*StoreDirResponse).Error; err == nil {
				delete(p.stores, Hexstr(key))
				p.storesMtx.Unlock()
			} else {
				//..
			}
		default:
			p.logger.Warn("Received unexpected message type")
		}
		conn.Close()
	}
}
