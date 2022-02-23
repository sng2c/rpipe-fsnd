package main

import (
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"fsnd/fsm"
	"fsnd/jobqueue"
	"fsnd/protocol"
	"github.com/dyninc/qstring"
	log "github.com/sirupsen/logrus"
	rpipe_msgspec "github.com/sng2c/rpipe/msgspec"
	"hash"
	"math"
	"net/url"
	"os"
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
	Job         *jobqueue.Job
	SessionId   string
	Origin      string
	RecvAddr    string `qstring:"addr,omitempty"`
	FileName    string
}

const BUFSIZE = 1024

func NewSendSessionFrom(job *jobqueue.Job) (*SendSession, error) {
	log.Debugln(1)
	str := strings.TrimSpace(string(job.Payload))
	values, err := url.ParseQuery(str)
	if err != nil {
		return nil, err
	}
	sess := SendSession{}
	sess.Job = job

	log.Debugln(2)
	err = qstring.Unmarshal(values, &sess)
	if err != nil {
		return nil, err
	}

	if sess.RecvAddr == "" {
		return nil, errors.New("Invalid payload " + str)
	}

	sess.LastSent = time.Now()
	sess.Origin = str
	sess.SessionId = job.JobName
	sess.FilePath = job.FilePath()
	sess.FileName = job.FileName

	log.Debugln(3)
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

	log.Debugln(4)
	//sess.SendProto.Fsm.LogDump()
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
		MsgV0: &rpipe_msgspec.ApplicationMsg{
			Name: sess.RecvAddr,
		},
		SrcType:   "SEND",
		SessionId: sess.SessionId,
		Event:     event,
		Length:    sess.ChunkLength,
	}
}
func (sess *SendSession) Handle(ackMsg *FsndMsg) (
	newMsg *FsndMsg,
	err error,
) {
	//log.Debugln(sess.Job)
	log.Debugf("SendSession %s to %s with %s", sess.FileName, sess.RecvAddr, sess.SessionId)
	if ok := sess.SendProto.Emit(ackMsg.Event); ok {

		if sess.SendProto.State == sess.LastState {
			err = LastError
		}

		newMsg = sess.NewFsndMsg(fsm.Event(sess.SendProto.State))

		cmd, _, page := protocol.ParseEventState(string(sess.SendProto.State))
		log.Debugf("State %s -> Cmd: %s , Page: %d", sess.SendProto.State, cmd, page)
		if cmd == "OPEN" {
			sess.FileObj, err = os.Open(sess.FilePath)
			sess.FileHash = readFileHash(sess.FilePath)
			newMsg.Hash = sess.FileHash
			newMsg.FileName = sess.FileName

		} else if cmd == "WRITE" {
			buf := make([]byte, BUFSIZE)
			offset := int64(page * BUFSIZE)
			log.Debugf("page %d, OFFSET %d ", page, offset)
			//seek, err := sess.FileObj.Seek(offset, io.SeekStart)
			//if err != nil {
			//	return nil, err
			//}

			var hasRead int
			hasRead, _ = sess.FileObj.ReadAt(buf, offset)
			log.Debugf("%s HasRead : %d", sess.FilePath, hasRead)
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
