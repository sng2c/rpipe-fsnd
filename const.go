package main

import (
	"errors"
	"fsnd/fsm"
	"fsnd/messages"
	"github.com/dyninc/qstring"
	log "github.com/sirupsen/logrus"
	"net/url"
	"strings"
)

var LastError error = errors.New("Last Error")

type FsndMsg struct {
	MsgV0     *messages.Msg `qstring:"-"`
	SrcType   string    `qstring:"type"`
	SessionId string    `qstring:"sid,omitempty"`
	Event     fsm.Event `qstring:"cmd,omitempty"`
	Hash      string    `qstring:"hash,omitempty"`
	FileName  string    `qstring:"file,omitempty"`
	DataB64   string    `qstring:"data,omitempty"`
	Length    int       `qstring:"len,omitempty"`
	Origin    string    `qstring:"-"`
}

func NewFsndMsgFrom(v0 *messages.Msg) (*FsndMsg, error) {
	str := strings.TrimSpace(string(v0.Data))
	values, err := url.ParseQuery(str)
	if err != nil {
		return nil, err
	}
	fsndMsg := FsndMsg{
		MsgV0: v0,
	}
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
	return msg.MsgV0.Marshal()
}

func (msg *FsndMsg) NewAck(event fsm.Event) *FsndMsg {
	log.Debugln(msg.MsgV0.From)
	return &FsndMsg{
		MsgV0:     &messages.Msg{To: msg.MsgV0.From},
		SessionId: msg.SessionId,
		Event:     event,
	}
}
