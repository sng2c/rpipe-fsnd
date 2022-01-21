package main

import (
	"fsnd/jobqueue"
	"io"
	"log"
	"os"
	"regexp"
	"time"
)

// session 을 만들것

var ver0MsgPat = regexp.MustCompile(`^([a-zA-Z0-9\\._-]+):(.+)$`)
var targetDir = "./out"
var tmpDir = "./out"
var jobQueueBase = "./queue"
var recvSess = make(map[string]*RecvSession)
var sendSess = make(map[string]*SendSession)

const TTL = 30

func main() {
	defer func() {
		log.Println("Bye")
	}()

	jobCh, err := jobqueue.StartJobQueue(jobQueueBase)
	if err != nil {
		return
	}
	log.Println("WATCHER")

	// RECEIVER
	stdin := RecvChannel(os.Stdin)
	//cancel, cancelFunc := context.WithCancel(ctx)
	prtCh := make(chan string)

	ticker := time.Tick(1 * time.Second)

LoopMain:
	for {
		select {
		case now := <-ticker:
			//log.Println("tick")
			for k, s := range recvSess {
				if s.IsTimeout(now, TTL){
					log.Printf("RecvSession %s is timeout", s.SessionKey())
					failMsg := s.NewFsndMsg("TIMEOUT")
					delete(recvSess, k)
					log.Println(failMsg)
					go func() { prtCh <- failMsg.Encode() }()
				}
			}
			for k, s := range sendSess {
				if s.IsTimeout(now, TTL){
					log.Printf("SendSession %s is timeout", s.SessionKey())
					failMsg := s.NewFsndMsg("TIMEOUT")
					delete(sendSess, k)
					log.Println(failMsg)
					go func() { prtCh <- failMsg.Encode() }()
				}
			}
		case output, ok := <-prtCh:
			if ok == false {
				return
			}
			os.Stdout.Write([]byte(output + "\n"))
		case job, ok := <-jobCh:
			if ok == false {
				break LoopMain
			}
			log.Println(job)
			job, err := job.MoveJob(jobqueue.StateDoing)
			if err != nil {
				log.Print(err)
				continue
			}
			log.Println(job)
			// process here
			sess, err := NewSendSessionFrom(job)
			if err != nil {
				log.Print(err)
				_, err := job.MoveJob(jobqueue.StateFailed)
				if err != nil {
					log.Print(err)
				}
				continue
			}
			sendSess[sess.SessionKey()] = sess
			sendMsg, err := sess.Handle(sess.NewFsndMsg("START"))

			if err != nil {
				if err != io.EOF {
					log.Print(err)
					sess.Job.MoveJob(jobqueue.StateFailed)
				} else {
					log.Printf("%s Done", sess.SessionKey())
					sess.Job.MoveJob(jobqueue.StateDone)
				}
				delete(sendSess, sess.SessionKey())
			}

			if sendMsg != nil {
				log.Println(sendMsg)
				go func() { prtCh <- sendMsg.Encode() }()
			}

		case line, ok := <-stdin:
			if ok == false {
				break LoopMain
			}
			log.Printf("[STDIN] %s", line)
			v0, err := ParseRpipeMsgV0(line)
			if err != nil {
				log.Println(err)
				continue
			}
			msg, err := NewFsndMsgFrom(v0)
			if err != nil {
				log.Println(err)
				continue
			}

			if msg.SrcType == "RECV"{
				var sess *SendSession
				sess, ok = sendSess[v0.Addr+msg.SessionId]

				if !ok { // 없으면 무시
					log.Printf("No session %s %s", v0.Addr+msg.SessionId, err)
					failMsg := FsndMsg{
						MsgV0:     RpipeMsgV0{
							Addr:    v0.Addr,
						},
						SessionId: msg.SessionId,
						Event:     "NO_SESSION_FAIL",
					}
					go func() { prtCh <- failMsg.Encode() }()
					continue
				}

				sendMsg, err := sess.Handle(msg)
				if err != nil {
					log.Println(err)

					if err == LastError {
						sess.Job.MoveJob(jobqueue.StateDone)
						log.Printf("%s Done", sess.SessionKey())
					} else {
						sess.Job.MoveJob(jobqueue.StateFailed)
					}
					delete(sendSess, sess.SessionKey())
				}
				if sendMsg != nil {
					log.Println(sendMsg)
					go func() { prtCh <- sendMsg.Encode() }()
				}
			} else {
				var sess *RecvSession
				sess, ok = recvSess[v0.Addr+msg.SessionId]

				if !ok {
					sess, err = NewRecvSessionFrom(*msg, targetDir)
					if err != nil {
						log.Print(err)
						continue
					}
					log.Printf("New Recv Session %s", sess.SessionKey())
					recvSess[sess.SessionKey()] = sess
				}

				newAck, err := sess.Handle(msg)

				if err != nil {
					log.Print(err)
					delete(recvSess, sess.SessionKey())

					if err == LastError {
						log.Printf("%s Done", sess.SessionKey())
					}

				} else if newAck != nil {
					log.Println(newAck)
					go func() { prtCh <- newAck.Encode() }()
				}
			}
		}
	}
}
