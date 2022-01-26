package main

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"fsnd/fsm"
	"fsnd/jobqueue"
	"fsnd/messages"
	"fsnd/protocol"
	"github.com/dyninc/qstring"
	log "github.com/sirupsen/logrus"
	"hash"
	"io"
	"math"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

type SendSession struct {
	SendProto   fsm.Instance
	LastState   fsm.State
	LastSent    time.Time
	FileObj     *os.File
	FileHash    string
	FilePath    string
	ChunkLength int
	HashObj     hash.Hash
	Job         jobqueue.Job
	SessionId   string
	Origin      string
	RecvAddr    string `qstring:"addr,omitempty"`
	FileName    string `qstring:"file,omitempty"`
}

const BUFSIZE = 4096

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
	sess.LastSent = time.Now()
	sess.Origin = str
	sess.SessionId = job.JobName
	sess.FilePath = path.Join(job.Path(), sess.FileName)

	fi, err := os.Stat(sess.FilePath)
	if err != nil {
		return nil, err
	}
	// get the size
	size := fi.Size()
	chunkLength := math.Ceil(float64(size) / float64(BUFSIZE))
	proto, lastState := protocol.NewSendProtocol(sess.SessionId, int(chunkLength))
	sess.SendProto = proto
	sess.LastState = lastState
	sess.ChunkLength = int(chunkLength)

	sess.SendProto.Fsm.LogDump()
	return &sess, nil
}
func (sess *SendSession) IsTimeout(now time.Time, ttl float64) bool {
	delta := now.Sub(sess.LastSent)
	return delta.Seconds() > ttl
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
func (sess *SendSession) NewFsndMsg(event fsm.Event) *FsndMsg {
	sess.LastSent = time.Now()
	return &FsndMsg{
		MsgV0: &messages.Msg{
			To: sess.RecvAddr,
		},
		SrcType:   "SEND",
		SessionId: sess.SessionId,
		Event:     event,
		Length: sess.ChunkLength,
	}
}
func (sess *SendSession) Handle(ackMsg *FsndMsg) (
	newMsg *FsndMsg,
	err error,
) {
	log.Debugln(sess)
	log.Debugf("SendSession %s to %s with %s", sess.FileName, sess.RecvAddr, sess.SessionId)
	if ok := sess.SendProto.Emit(ackMsg.Event); ok {

		if sess.SendProto.State == sess.LastState {
			err = LastError
		}

		newMsg = sess.NewFsndMsg(fsm.Event(sess.SendProto.State))

		cmd, _, page := protocol.ParseEventState(string(sess.SendProto.State))
		if cmd == "OPEN" {
			sess.FileObj, err = os.Open(sess.FilePath)
			sess.FileHash = readFileHash(sess.FilePath)
			newMsg.Hash = sess.FileHash
			newMsg.FileName = sess.FileName

		} else if cmd == "WRITE" {
			buf := make([]byte, BUFSIZE)
			offset := int64(page * BUFSIZE)
			log.Debugf("page %d, OFFSET %d ", page, offset)
			sess.FileObj.Seek(offset, io.SeekStart)

			var hasRead int
			hasRead, _ = sess.FileObj.Read(buf)
			buf = buf[:hasRead]
			newMsg.Event = fsm.Event(sess.SendProto.State)
			newMsg.DataB64 = base64.StdEncoding.EncodeToString(buf)
		} else if cmd == "CLOSE" {
			sess.FileObj.Close()
		}
	} else {
		newMsg = sess.NewFsndMsg("STATE_FAIL")
		err = LastError
	}

	return
}
