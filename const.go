package main

import (
	"errors"
	"fsnd/fsm"
	"github.com/dyninc/qstring"
	log "github.com/sirupsen/logrus"
	rpipe_msgspec "github.com/sng2c/rpipe/msgspec"
	"net/url"
	"strings"
)

var LastError error = errors.New("Last Error")

type FsndMsg struct {
	MsgV0     *rpipe_msgspec.ApplicationMsg `qstring:"-"`
	SrcType   string                        `qstring:"type"`
	SessionId string                        `qstring:"sid,omitempty"`
	Event     fsm.Event                     `qstring:"cmd,omitempty"`
	Hash      string                        `qstring:"hash,omitempty"`
	FileName  string                        `qstring:"file,omitempty"`
	DataB64   string                        `qstring:"data,omitempty"`
	Length    int                           `qstring:"len,omitempty"`
	From      string                        `qstring:"from,omitempty"`
	Origin    string                        `qstring:"-"`
}

func NewFsndMsgFrom(v0 *rpipe_msgspec.ApplicationMsg) (*FsndMsg, error) {
	str := strings.TrimSpace(string(v0.Data))
	values, err := url.ParseQuery(str)
	if err != nil {
		return nil, err
	}
	fsndMsg := FsndMsg{
		MsgV0: v0,
	}
	fsndMsg.From = v0.Name
	err = qstring.Unmarshal(values, &fsndMsg)
	if err != nil {
		return nil, err
	}
	fsndMsg.Origin = str
	return &fsndMsg, nil
}

func (msg *FsndMsg) Encode() []byte {
	marshalString, err := qstring.MarshalString(msg)
	if err != nil {
		return nil
	}
	msg.MsgV0.Data = []byte(marshalString)
	return msg.MsgV0.Encode()
}

func (msg *FsndMsg) NewAck(event fsm.Event) *FsndMsg {
	log.Debugln(msg.MsgV0.Name)
	return &FsndMsg{
		MsgV0:     &rpipe_msgspec.ApplicationMsg{Name: msg.MsgV0.Name},
		SessionId: msg.SessionId,
		Event:     event,
	}
}
