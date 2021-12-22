package main

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"hash"
	"log"
	"os"
	"path"
)

type RecvSession struct {
	LastSeq    int
	FileObj    *os.File
	TargetBase string
	HashObj    hash.Hash
	SessionId  string
	SenderAddr string
	FileName   string
	Hash       string
}

func NewRecvSessionFrom(msg FsndMsg, targetBase string) (*RecvSession, error) {
	if msg.Command != CmdFileOpen {
		return nil, errors.New("Command is not CmdFileOpen")
	}
	sess := RecvSession{
		LastSeq:    0,
		FileObj:    nil,
		TargetBase: targetBase,
		HashObj:    md5.New(),
		SessionId:  msg.SessionId,
		SenderAddr: msg.MsgV0.Addr,
		FileName:   msg.FileName,
		Hash:       msg.Hash,
	}
	return &sess, nil
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
func (sess *RecvSession) Handle(msg *FsndMsg) (*FsndMsg, bool, error) {
	var err error
	if msg.Command != CmdFileOpen && sess.LastSeq >= msg.Seq {
		err := errors.New(fmt.Sprintf("Seq is invalid (lastSeq %d >= msg.Seq %d)", sess.LastSeq, msg.Seq))
		log.Print(err)
		return nil, false, err
	} else {
		sess.LastSeq = msg.Seq
	}

	err = os.MkdirAll(sess.SessionPath(), 0755)
	if err != nil {
		log.Println(err)
		return nil, false, err
	}
	switch msg.Command {
	case CmdFileOpen:
		err = os.WriteFile(sess.JobPath(), []byte(msg.Origin), 0600)
		if err != nil {
			log.Println(err)
			return nil, false, err
		}
		sess.FileObj, err = os.Create(sess.FilePath())
		if err != nil {
			log.Println(err)
			return nil, false, err
		}
	case CmdFileWrite:
		decoded, err := base64.StdEncoding.DecodeString(msg.DataB64)
		if err != nil {
			log.Println(err)
			return nil, false, err
		}
		sess.HashObj.Write(decoded)
		written, err := sess.FileObj.Write(decoded)
		if err != nil {
			log.Println(err)
			return nil, false, err
		}
		log.Printf("Written %d", written)
	case CmdFileClose:
		err = sess.FileObj.Close()
		if err != nil {
			log.Println(err)
			return nil, false, err
		}
		hstr := fmt.Sprintf("%x", sess.HashObj.Sum(nil))
		if hstr != sess.Hash {
			err := errors.New(fmt.Sprintf("Hash is not match %s != %s", sess.Hash, hstr))
			log.Println(err)
			err = os.Remove(sess.FilePath())
			if err != nil {
				log.Println(err)
				return nil, false, err
			}
			return nil, false, err
		}
		log.Println("Done")
		return msg.NewOk(), false, nil
	}
	return msg.NewAck(), true, nil
}
