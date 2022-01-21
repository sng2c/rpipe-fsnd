package main

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"fsnd/fsm"
	"fsnd/protocol"
	"hash"
	"log"
	"os"
	"path"
	"time"
)

type RecvSession struct {
	RecvProto  fsm.Instance
	LastSent   time.Time
	FileObj    *os.File
	TargetBase string
	HashObj    hash.Hash
	SessionId  string
	SenderAddr string
	FileName   string
	Hash       string
	LastState  fsm.State
}

func NewRecvSessionFrom(msg FsndMsg, targetBase string) (*RecvSession, error) {
	proto, lastState := protocol.NewRecvProtocol(msg.SessionId, msg.Length)
	sess := RecvSession{
		RecvProto:  proto,
		LastState:  lastState,
		LastSent:   time.Now(),
		FileObj:    nil,
		TargetBase: targetBase,
		HashObj:    md5.New(),
		SessionId:  msg.SessionId,
		SenderAddr: msg.MsgV0.Addr,
		FileName:   msg.FileName,
		Hash:       msg.Hash,
	}
	proto.Fsm.LogDump()
	return &sess, nil
}
func (sess *RecvSession) NewFsndMsg(event fsm.Event) *FsndMsg {
	sess.LastSent = time.Now()
	newMsg := &FsndMsg{
		MsgV0: RpipeMsgV0{
			Addr: sess.SenderAddr,
		},
		SrcType:   "RECV",
		SessionId: sess.SessionId,
		Event:     event,
	}
	return newMsg
}
func (sess *RecvSession) IsTimeout(now time.Time, ttl float64) bool {
	delta := now.Sub(sess.LastSent)
	return delta.Seconds() > ttl
}
func (sess *RecvSession) SessionKey() string {
	return sess.SenderAddr + sess.SessionId
}
func (sess *RecvSession) SessionPath() string {
	return path.Join(targetDir, sess.SessionId)
}
func (sess *RecvSession) FilePath() string {
	return path.Join(sess.SessionPath(), sess.FileName)
}
func (sess *RecvSession) JobPath() string {
	return path.Join(sess.SessionPath(), "job.txt")
}
func (sess *RecvSession) Handle(msg *FsndMsg) (newMsg *FsndMsg, _err error) {
	{
		err := os.MkdirAll(sess.SessionPath(), 0755)
		if err != nil {
			_err = err
			log.Println(err)
			newMsg = sess.NewFsndMsg("MKDIR_FAIL")
		}
	}
	if ok := sess.RecvProto.Emit(msg.Event); ok {

		if sess.RecvProto.State == sess.LastState {
			_err = LastError
		}

		cmd, _, _ := protocol.ParseEventState(string(sess.RecvProto.State))
		if cmd == "OPENED" {
			err := os.WriteFile(sess.JobPath(), []byte(msg.Origin), 0600)
			if err != nil {
				_err = err
				log.Println(err)
			}
			sess.FileObj, _ = os.Create(sess.FilePath())
		} else if cmd == "WRITTEN" {
			decoded, err := base64.StdEncoding.DecodeString(msg.DataB64)
			if err != nil {
				_err = err
				log.Println(err)
			}
			sess.HashObj.Write(decoded)
			written, _ := sess.FileObj.Write(decoded)
			log.Printf("Written %d", written)
		} else if cmd == "CLOSED" {
			err := sess.FileObj.Close()
			if err != nil {
				_err = err
				log.Println(err)
			}
			hstr := fmt.Sprintf("%x", sess.HashObj.Sum(nil))
			if hstr != sess.Hash {
				_err = errors.New(fmt.Sprintf("Hash is not match %s != %s", sess.Hash, hstr))
				log.Println(_err)
				os.Remove(sess.FilePath())
			}
			log.Println("Done")

		}
		if _err != nil {
			newMsg = sess.NewFsndMsg(fsm.Event(_err.Error()))
		} else {
			newMsg = sess.NewFsndMsg(fsm.Event(sess.RecvProto.State))
		}
	} else {
		newMsg = sess.NewFsndMsg("STATE_FAIL")
		_err = LastError
	}

	return
}
