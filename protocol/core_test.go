package protocol

import (
	"fsnd/fsm"
	"testing"
)

func TestRecv(t *testing.T) {
	var ok bool
	m, _ := NewRecvProtocol("A", 3)
	if ok = m.Emit("OPEN.A."); !ok {
		t.Fail()
	}
	if m.State != "OPENED.A." {
		t.Fail()
	}
	m.Fsm.LogDump()

	if ok = m.Emit("WRITE.A.0"); !ok {
		t.Fail()
	}
	if m.State != "WRITTEN.A.0" {
		t.Fail()
	}

	if ok = m.Emit("CLOSE"); ok {
		t.Fail()
	}
}

func TestSend(t *testing.T) {
	var ok bool
	m, _ := NewSendProtocol("A", 3)
	if ok = m.Emit("START"); !ok {
		t.Fail()
	}
	if m.State != "OPEN.A." {
		t.Fail()
	}
	if ok = m.Emit("OPENED.A."); !ok {
		t.Fail()
	}
	if m.State != "WRITE.A.0" {
		t.Fail()
	}
	m.Fsm.LogDump()

	if ok = m.Emit("WRITTEN.A.0"); !ok {
		t.Fail()
	}
	if m.State != "WRITE.A.1" {
		t.Fail()
	}

	if ok = m.Emit("CLOSE"); ok {
		t.Fail()
	}
}

func TestCross(t *testing.T) {
	id := "AAA"
	chunks := 5
	send, _ := NewSendProtocol(id, chunks)
	send.Fsm.LogDump()
	recv, recvFinalState := NewRecvProtocol(id, chunks)
	recv.Fsm.LogDump()

	var sendEvent fsm.Event
	var recvEvent fsm.Event

	var ok bool

	recvEvent = fsm.Event("START")
	for {
		t.Logf("Send(%-15s) %20s Recv(%-15s)", send.State, "<--"+recvEvent+"--", recv.State)
		if ok = send.Emit(recvEvent); !ok {
			t.Errorf("Invalid recvEvent(%s)", recvEvent)
			t.FailNow()
		}
		sendEvent = fsm.Event(send.State)


		t.Logf("Send(%-15s) %-20s Recv(%-15s)", send.State, "--"+sendEvent+"-->", recv.State)
		if ok = recv.Emit(sendEvent); !ok {
			t.Errorf("Invalid sendEvent(%s)", sendEvent)
			t.FailNow()
		}
		recvEvent = fsm.Event(recv.State)
		if recv.State == recvFinalState {
			break
		}
	}
}

func TestParseEventState(t *testing.T) {
	type args struct {
		ev_st string
	}
	tests := []struct {
		name     string
		args     args
		wantCmd  string
		wantSid  string
		wantPage int
	}{
		// TODO: Add test cases.
		{
			name:     "1",
			args:     args{"A.B.1"},
			wantCmd:  "A",
			wantSid:  "B",
			wantPage: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCmd, gotSid, gotPage := ParseEventState(tt.args.ev_st)
			if gotCmd != tt.wantCmd {
				t.Errorf("ParseEventState() gotCmd = %v, want %v", gotCmd, tt.wantCmd)
			}
			if gotSid != tt.wantSid {
				t.Errorf("ParseEventState() gotSid = %v, want %v", gotSid, tt.wantSid)
			}
			if gotPage != tt.wantPage {
				t.Errorf("ParseEventState() gotPage = %v, want %v", gotPage, tt.wantPage)
			}
		})
	}
}