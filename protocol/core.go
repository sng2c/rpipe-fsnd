package protocol

import (
	"fsnd/fsm"
	"strconv"
	"strings"
)

func init() {

}

func NewRecvProtocol(id string, chunks int) (fsm.Instance, fsm.State) {
	id = "." + id + "."
	recv := fsm.NewFsm()
	recv.Append(fsm.State("READY"+id), fsm.Event("OPEN"+id), fsm.State("OPENED"+id))
	recv.Append(fsm.State("OPENED"+id), fsm.Event("WRITE"+id+"0"), fsm.State("WRITTEN"+id+"0"))
	for i := 1; i < chunks; i++ {
		recv.Append(
			fsm.State("WRITTEN"+id+strconv.Itoa(i-1)),
			fsm.Event("WRITE"+id+strconv.Itoa(i)),
			fsm.State("WRITTEN"+id+strconv.Itoa(i)))
	}
	recv.Append(fsm.State("WRITTEN"+id+strconv.Itoa(chunks-1)), fsm.Event("CLOSE"+id), fsm.State("CLOSED"+id))
	return recv.NewInstance(fsm.State("READY" + id)), fsm.State("CLOSED" + id)
}

func NewSendProtocol(id string, chunks int) (fsm.Instance, fsm.State) {
	id = "." + id + "."
	send := fsm.NewFsm()
	send.Append(fsm.State("READY"), fsm.Event("START"), fsm.State("OPEN"+id))
	send.Append(fsm.State("OPEN"+id), fsm.Event("OPENED"+id), fsm.State("WRITE"+id+"0"))
	for i := 0; i < chunks-1; i++ {
		send.Append(
			fsm.State("WRITE"+id+strconv.Itoa(i)),
			fsm.Event("WRITTEN"+id+strconv.Itoa(i)),
			fsm.State("WRITE"+id+strconv.Itoa(i+1)))
	}
	send.Append(
		fsm.State("WRITE"+id+strconv.Itoa(chunks-1)),
		fsm.Event("WRITTEN"+id+strconv.Itoa(chunks-1)),
		fsm.State("CLOSE"+id))

	return send.NewInstance(fsm.State("READY")), fsm.State("CLOSE" + id)
}

func ParseEventState(ev_st string) (cmd string, sid string, page int) {
	chunks := strings.Split(ev_st, ".")
	if len(chunks) > 0 {
		cmd = chunks[0]
	}
	if len(chunks) > 1 {
		sid = chunks[1]
	}
	if len(chunks) > 2 {
		_page, err := strconv.Atoi(chunks[2])
		if err == nil {
			page = _page
		}
	}
	return
}
