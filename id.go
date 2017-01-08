package p2p

import (
	"bytes"
	"encoding/hex"
	"github.com/pkg/errors"
)

const ID_LENGTH = 20
const ID_SIZE = ID_LENGTH * 8

// Peer Id
// Hash(peer_addr, router_addr) for now..
type Id [ID_LENGTH]byte

func NewId(data []byte) (id Id) {
	h := Hash(data)
	copy(id[:], h)
	return
}

func (id Id) String() string {
	return Hexstr(id[:])
}

func (id Id) FromString(str string) error {
	data, err := hex.DecodeString(str)
	if err != nil {
		return err
	}
	if len(data) != ID_LENGTH {
		return errors.Errorf("Expected data with length=%d; got data with length=%d", ID_LENGTH, len(data))
	}
	copy(id[:], data)
	return nil
}

func IdFromString(str string) (id Id, err error) {
	err = id.FromString(str)
	return
}

func (id Id) Equals(other Id) bool {
	return bytes.Equal(id[:], other[:])
}

func (id Id) Less(other Id) bool {
	return bytes.Compare(id[:], other[:]) == -1
}

type Ids []Id

func (ids Ids) Len() int           { return len(ids) }
func (ids Ids) Less(i, j int) bool { return ids[i].Less(ids[j]) }
func (ids Ids) Swap(i, j int)      { ids[i], ids[j] = ids[j], ids[i] }
