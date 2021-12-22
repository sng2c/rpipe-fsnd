package main

import (
	"fsnd/jobqueue"
	"io"
	"log"
	"os"
	"regexp"
)

// session 을 만들것

var ver0MsgPat = regexp.MustCompile(`^([a-zA-Z0-9\\._-]+):(.+)$`)
var targetDir = "./out"
var tmpDir = "./out"
var jobQueueBase = "./queue"
var recvSess = make(map[string]*RecvSession)
var sendSess = make(map[string]*SendSession)

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

LoopMain:
	for {
		select {
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
			sendMsg, doNext, err := sess.Handle(nil)

			if !doNext {
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

			if msg.Command == CmdAck || msg.Command == CmdOk {
				var sess *SendSession
				sess, ok = sendSess[v0.Addr+msg.SessionId]

				if !ok { // 없으면 무시
					log.Printf("No session %s %s", v0.Addr+msg.SessionId, err)
					continue
				}

				sendMsg, doNext, err := sess.Handle(msg)
				if err != nil {
					log.Println(err)
				}
				if doNext == false {
					log.Printf("%s Done", sess.SessionKey())
					_, _ = sess.Job.MoveJob(jobqueue.StateDone)
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

				newAck, doNext, err := sess.Handle(msg)

				if err != nil {
					log.Print(err)
				}
				if doNext == false {
					delete(recvSess, sess.SessionKey())
					log.Printf("%s Done", sess.SessionKey())
				}
				if newAck != nil {
					log.Println(newAck)
					go func() { prtCh <- newAck.Encode() }()
				}
			}
		}
	}
}
