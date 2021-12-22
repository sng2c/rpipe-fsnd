package main

import (
	"errors"
	"fmt"
	"github.com/dyninc/qstring"
	"log"
	"net/url"
	"strings"
)

type Command string

const (
	CmdAck       = Command("ACK")
	CmdFileOpen  = Command("OPEN")
	CmdFileWrite = Command("WRITE")
	CmdFileClose = Command("CLOSE")
	CmdOk        = Command("OK")
	CmdFail      = Command("FAIL")
)

// TODO rpipe 프로젝트에 모듈로 import 할 수 있게 넣을 것
type RpipeMsgV0 struct {
	Addr    string `qstring:"-"`
	Payload string `qstring:"-"`
}

func (v0 RpipeMsgV0) Encode() string {
	return fmt.Sprintf("%s:%s", v0.Addr, v0.Payload)
}
func ParseRpipeMsgV0(str string) (RpipeMsgV0, error) {
	matched := ver0MsgPat.FindStringSubmatch(str)
	if len(matched) != 3 {
		err := errors.New("Invalid format " + str)
		return RpipeMsgV0{}, err
	}
	return RpipeMsgV0{
		Addr:    matched[1],
		Payload: matched[2],
	}, nil
}

type FsndMsg struct {
	MsgV0     RpipeMsgV0
	SessionId string  `qstring:"sid,omitempty"`
	Command   Command `qstring:"cmd,omitempty"`
	Hash      string  `qstring:"hash,omitempty"`
	FileName  string  `qstring:"file,omitempty"`
	DataB64   string  `qstring:"data,omitempty"`
	Seq       int     `qstring:"seq"`
	Origin    string  `qstring:"-"`
}

func NewFsndMsgFrom(v0 RpipeMsgV0) (*FsndMsg, error) {
	str := strings.TrimSpace(v0.Payload)
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

func (msg *FsndMsg) Encode() string {
	marshalString, err := qstring.MarshalString(msg)
	if err != nil {
		return ""
	}
	msg.MsgV0.Payload = marshalString
	return msg.MsgV0.Encode()
}

func (msg *FsndMsg) NewAck() *FsndMsg {
	log.Println(msg.MsgV0.Addr)
	return &FsndMsg{
		MsgV0:     RpipeMsgV0{Addr: msg.MsgV0.Addr},
		SessionId: msg.SessionId,
		Command:   CmdAck,
		Seq:       msg.Seq,
	}
}
func (msg *FsndMsg) NewOk() *FsndMsg {
	log.Println(msg.MsgV0.Addr)
	return &FsndMsg{
		MsgV0:     RpipeMsgV0{Addr: msg.MsgV0.Addr},
		SessionId: msg.SessionId,
		Command:   CmdOk,
		Seq:       msg.Seq,
	}
}
