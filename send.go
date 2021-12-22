package main

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"fsnd/jobqueue"
	"github.com/dyninc/qstring"
	"hash"
	"io"
	"log"
	"net/url"
	"os"
	"path"
	"strings"
)

type SendSession struct {
	LastSeq   int
	FileObj   *os.File
	FileHash  string
	FilePath  string
	HashObj   hash.Hash
	Job       jobqueue.Job
	SessionId string
	Origin    string
	RecvAddr  string `qstring:"addr,omitempty"`
	FileName  string `qstring:"file,omitempty"`
}

func NewSendSessionFrom(job jobqueue.Job) (*SendSession, error) {
	str := strings.TrimSpace(job.Payload)
	values, err := url.ParseQuery(str)
	if err != nil {
		return nil, err
	}
	sess := SendSession{}
	sess.Job = job
	err = qstring.Unmarshal(values, &sess)
	if err != nil {
		return nil, err
	}

	if sess.RecvAddr == "" || sess.FileName == "" {
		return nil, errors.New("Invalid payload " + str)
	}

	sess.Origin = str
	sess.SessionId = job.JobName
	sess.FilePath = path.Join(job.Path(), sess.FileName)
	return &sess, nil
}

func readFileHash(path string) string {
	h := md5.New()
	file, err := os.ReadFile(path)

	if err != nil {
		return ""
	}
	h.Write(file)
	return fmt.Sprintf("%x", h.Sum(nil))
}
func (sess *SendSession) SessionKey() string {
	return sess.RecvAddr + sess.SessionId
}

func (sess *SendSession) Handle(ackMsg *FsndMsg) (
	newMsg *FsndMsg,
	doMore bool,
	err error,
) {
	log.Printf("SendSession %s to %s with %s", sess.FileName, sess.RecvAddr, sess.SessionId)
	doMore = true
	if ackMsg != nil && ackMsg.Seq != sess.LastSeq {
		doMore = false
		err = errors.New("Invalid Seq")
		newMsg = &FsndMsg{
			MsgV0: RpipeMsgV0{
				Addr: sess.RecvAddr,
			},
			SessionId: sess.SessionId,
			Command:   CmdFail,
			Seq:       sess.LastSeq + 1,
		}
		return
	}

	sess.LastSeq += 1

	if ackMsg != nil && ackMsg.Command == CmdOk {
		return nil, false, nil
	} else {
		if sess.FileObj == nil {
			sess.FileObj, err = os.Open(sess.FilePath)
			sess.FileHash = readFileHash(sess.FilePath)
			if err != nil {
				return nil, false, err
			}
			newMsg = &FsndMsg{
				MsgV0: RpipeMsgV0{
					Addr: sess.RecvAddr,
				},
				SessionId: sess.SessionId,
				Command:   CmdFileOpen,
				Hash:      sess.FileHash,
				FileName:  sess.FileName,
				Seq:       sess.LastSeq,
			}

			return
		} else {
			bufSize := 4096
			buf := make([]byte, bufSize)
			var hasRead int
			hasRead, err = sess.FileObj.Read(buf)
			buf = buf[:hasRead]
			if err != nil {
				if err == io.EOF {
					doMore = true
					newMsg = &FsndMsg{
						MsgV0: RpipeMsgV0{
							Addr: sess.RecvAddr,
						},
						SessionId: sess.SessionId,
						Command:   CmdFileClose,
						Seq:       sess.LastSeq,
					}
					return
				} else {
					doMore = false
				}
				return
			}
			newMsg = &FsndMsg{
				MsgV0: RpipeMsgV0{
					Addr: sess.RecvAddr,
				},
				SessionId: sess.SessionId,
				Command:   CmdFileWrite,
				DataB64:   base64.StdEncoding.EncodeToString(buf),
				Seq:       sess.LastSeq,
			}
			return
		}
	}

}
