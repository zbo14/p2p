package p2p

import (
	"encoding/gob"
	"encoding/json"
	"io"
)

type Message interface {
	IsMessage()
	From() Id
	Key() []byte
	Timestamp() int64
	To() Id
}

type Header struct {
	From_      Id     `json:"from"`
	Key_       []byte `json:"key"`
	Timestamp_ int64  `json:"timestamp"`
	To_        Id     `json:"to"`
}

func NewHeader(from Id, key []byte, to Id) Header {
	return Header{from, key, Timestamp(), to}
}

func (h Header) From() Id         { return h.From_ }
func (h Header) Key() []byte      { return h.Key_ }
func (h Header) Timestamp() int64 { return h.Timestamp_ }
func (h Header) To() Id           { return h.To_ }

type FindFileRequest struct {
	Header `json:"header"`
}

type FindFileResponse struct {
	Error    error `json:"error"`
	Filesize int64 `json:"file_size"`
	Header   `json:"header"`
	Target   Id `json:"target"`
}

type StoreFileRequest struct {
	Filesize int64 `json:"file_size"`
	Header   `json:"header"`
}

type StoreFileResponse struct {
	Error  error `json:"error"`
	Header `json:"header"`
}

type FindDirRequest struct {
	Header `json:"header"`
}

type FindDirResponse struct {
	Error     error    `json:"error"`
	Filenames []string `json:"file_names"`
	Filesizes []int64  `json:"file_sizes"`
	Header    `json:"header"`
	Target    Id `json:"target"`
}

type StoreDirRequest struct {
	Filenames []string `json:"file_names"`
	Filesizes []int64  `json:"file_sizes"`
	Header    `json:"header"`
}

type StoreDirResponse struct {
	Error  error `json:"error"`
	Header `json:"header"`
}

func (_ *FindFileRequest) IsMessage()   {}
func (_ *FindFileResponse) IsMessage()  {}
func (_ *StoreFileRequest) IsMessage()  {}
func (_ *StoreFileResponse) IsMessage() {}
func (_ *FindDirRequest) IsMessage()    {}
func (_ *FindDirResponse) IsMessage()   {}
func (_ *StoreDirRequest) IsMessage()   {}
func (_ *StoreDirResponse) IsMessage()  {}

func NewFindFileRequest(from Id, key []byte, to Id) *FindFileRequest {
	return &FindFileRequest{
		Header: NewHeader(from, key, to),
	}
}

func NewFindFileResponse(err error, filesize int64, from Id, key []byte, target, to Id) *FindFileResponse {
	return &FindFileResponse{
		Error:    err,
		Filesize: filesize,
		Header:   NewHeader(from, key, to),
		Target:   target,
	}
}

func NewStoreFileRequest(id Id, filesize int64, key []byte) *StoreFileRequest {
	return &StoreFileRequest{
		Filesize: filesize,
		Header:   NewHeader(id, key, id), //to == from
	}
}

func NewStoreFileResponse(err error, from Id, key []byte, to Id) *StoreFileResponse {
	return &StoreFileResponse{
		Error:  err,
		Header: NewHeader(from, key, to),
	}
}

func NewFindDirRequest(from Id, key []byte, to Id) *FindDirRequest {
	return &FindDirRequest{
		Header: NewHeader(from, key, to),
	}
}

func NewFindDirResponse(err error, filenames []string, filesizes []int64, from Id, key []byte, target, to Id) *FindDirResponse {
	return &FindDirResponse{
		Error:     err,
		Filenames: filenames,
		Filesizes: filesizes,
		Header:    NewHeader(from, key, to),
		Target:    target,
	}
}

func NewStoreDirRequest(id Id, filenames []string, filesizes []int64, key []byte) *StoreDirRequest {
	return &StoreDirRequest{
		Filenames: filenames,
		Filesizes: filesizes,
		Header:    NewHeader(id, key, id),
	}
}

func NewStoreDirResponse(err error, from Id, key []byte, to Id) *StoreDirResponse {
	return &StoreDirResponse{
		Error:  err,
		Header: NewHeader(from, key, to),
	}
}

var EncodeMessage = GobEncodeMessage
var DecodeMessage = GobDecodeMessage

// Gob

func GobEncodeMessage(msg Message, w io.Writer) error {
	enc := gob.NewEncoder(w)
	if err := enc.Encode(msg); err != nil {
		return err
	}
	return nil
}

func GobDecodeMessage(r io.Reader) (msg Message, err error) {
	dec := gob.NewDecoder(r)
	if err = dec.Decode(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

// JSON

func JSONEncodeMessage(msg Message, w io.Writer) error {
	enc := json.NewEncoder(w)
	if err := enc.Encode(msg); err != nil {
		return err
	}
	return nil
}

func JSONDecodeMessage(r io.Reader) (msg Message, err error) {
	dec := json.NewDecoder(r)
	if err = dec.Decode(msg); err != nil {
		return nil, err
	}
	return msg, nil
}

type Messages []Message

func (msgs Messages) Len() int {
	return len(msgs)
}

func (msgs Messages) Less(i, j int) bool {
	// So newest messages are first
	return msgs[i].Timestamp() > msgs[j].Timestamp()
}

func (msgs Messages) Swap(i, j int) {
	msgs[i], msgs[j] = msgs[j], msgs[i]
}
