package p2p

import "encoding/gob"

func RegisterTypes() {
	gob.Register(DirHeader{})
	gob.Register(FileHeader{})
	gob.Register(FindFileRequest{})
	gob.Register(FindFileResponse{})
	gob.Register(StoreFileRequest{})
	gob.Register(StoreFileResponse{})
	gob.Register(FindDirRequest{})
	gob.Register(FindDirResponse{})
	gob.Register(StoreDirRequest{})
	gob.Register(StoreDirResponse{})
	gob.Register(LeaveNetworkArgs{})
	gob.Register(LeaveNetworkReply{})
	gob.Register(CopyTableArgs{})
	gob.Register(CopyTableReply{})
	gob.Register(FindContactArgs{})
	gob.Register(FindContactReply{})
	gob.Register(Ping{})
	gob.Register(Pong{})
}
