package main

import (
	"flag"
	"fsnd/jobqueue"
	rpipe_msgspec "github.com/sng2c/rpipe/msgspec"
	rpipe_pipe "github.com/sng2c/rpipe/pipe"
	easy "github.com/t-tomalak/logrus-easy-formatter"
	"io"
	"os"
	"time"
)
import (
	log "github.com/sirupsen/logrus"
)

// session 을 만들것
const VERSION = "0.1.2"

var recvSess = make(map[string]*RecvSession)
var sendSess = make(map[string]*SendSession)

const TTL = 30

func main() {
	defer func() {
		log.Debugln("Bye")
	}()

	var verbose bool

	var downloadPath string
	var uploadPath string

	flag.BoolVar(&verbose, "verbose", false, "Verbose")
	flag.StringVar(&downloadPath, "download", "./download", "Download directory")
	flag.StringVar(&uploadPath, "upload", "./upload", "Upload directory")
	flag.Parse()

	if verbose {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}

	log.SetFormatter(&easy.Formatter{
		LogFormat: "%msg%",
	})
	log.Printf("Fsnd V%s\n", VERSION)
	log.Printf("  download  : %s\n", downloadPath)
	log.Printf("  upload    : %s\n", uploadPath)
	log.Printf("  verbose   : %t\n", verbose)
	log.SetFormatter(&log.TextFormatter{ForceColors: true})

	jobCh, err := jobqueue.StartJobQueue(uploadPath)
	if err != nil {
		return
	}
	log.Debugln("WATCHER")

	// RECEIVER
	stdin := rpipe_pipe.ReadLineChannel(os.Stdin)
	//cancel, cancelFunc := context.WithCancel(ctx)
	prtCh := make(chan []byte)

	ticker := time.Tick(1 * time.Second)

LoopMain:
	for {
		select {
		case now := <-ticker:
			//log.Debugln("tick")
			for k, s := range recvSess {
				if s.IsTimeout(now, TTL) {
					log.Debugf("RecvSession %s is timeout", s.SessionKey())
					failMsg := s.NewFsndMsg("TIMEOUT")
					delete(recvSess, k)
					log.Debugln(failMsg)
					go func() { prtCh <- failMsg.Encode() }()
				}
			}
			for k, s := range sendSess {
				if s.IsTimeout(now, TTL) {
					log.Debugf("SendSession %s is timeout", s.SessionKey())
					failMsg := s.NewFsndMsg("TIMEOUT")
					_, _ = s.Job.MoveJob(jobqueue.StateFailed)
					delete(sendSess, k)
					log.Debugln(failMsg)
					go func() { prtCh <- failMsg.Encode() }()
				}
			}
		case output, ok := <-prtCh:
			if ok == false {
				return
			}
			os.Stdout.Write(append(output, '\n'))
		case job, ok := <-jobCh:
			if ok == false {
				break LoopMain
			}
			log.Debugln(job)
			job, err := job.MoveJob(jobqueue.StateDoing)
			if err != nil {
				log.Debug(err)
				continue
			}
			log.Debugln(job)
			// process here
			sess, err := NewSendSessionFrom(job)
			if err != nil {
				log.Debug(err)
				_, err := job.MoveJob(jobqueue.StateFailed)
				if err != nil {
					log.Debug(err)
				}
				continue
			}
			sendSess[sess.SessionKey()] = sess
			sendMsg, err := sess.Handle(sess.NewFsndMsg("START"))

			if err != nil {
				if err != io.EOF {
					log.Debug(err)
					sess.Job.MoveJob(jobqueue.StateFailed)
				} else {
					log.Debugf("%s Done", sess.SessionKey())
					sess.Job.MoveJob(jobqueue.StateDone)
				}
				delete(sendSess, sess.SessionKey())
			}

			if sendMsg != nil {
				log.Debugln(sendMsg)
				go func() { prtCh <- sendMsg.Encode() }()
			}

		case line, ok := <-stdin:
			if ok == false {
				break LoopMain
			}
			log.Debugf("[STDIN] %s", line)
			v0, err := rpipe_msgspec.NewMsgFromBytes(line)

			msg, err := NewFsndMsgFrom(v0)
			if err != nil {
				log.Debugln(err)
				continue
			}

			if msg.SrcType == "RECV" {
				var sess *SendSession
				sess, ok = sendSess[v0.From+msg.SessionId]

				if !ok { // 없으면 무시
					log.Debugf("No session %s %s", v0.From+msg.SessionId, err)
					failMsg := FsndMsg{
						MsgV0: &rpipe_msgspec.Msg{
							To: v0.From,
						},
						SessionId: msg.SessionId,
						Event:     "NO_SESSION_FAIL",
					}
					go func() { prtCh <- failMsg.Encode() }()
					continue
				}

				sendMsg, err := sess.Handle(msg)
				if err != nil {
					log.Debugln(err)

					if err == LastError {
						sess.Job.MoveJob(jobqueue.StateDone)
						log.Debugf("%s Done", sess.SessionKey())
					} else {
						sess.Job.MoveJob(jobqueue.StateFailed)
					}
					delete(sendSess, sess.SessionKey())
				}
				if sendMsg != nil {
					log.Debugln(sendMsg)
					go func() { prtCh <- sendMsg.Encode() }()
				}
			} else {
				var sess *RecvSession
				sess, ok = recvSess[v0.From+msg.SessionId]

				if !ok {
					sess, err = NewRecvSessionFrom(*msg, downloadPath)
					if err != nil {
						log.Debug(err)
						continue
					}
					log.Debugf("New Recv Session %s", sess.SessionKey())
					recvSess[sess.SessionKey()] = sess
				}

				newAck, err := sess.Handle(msg)

				if err != nil {
					log.Debug(err)
					delete(recvSess, sess.SessionKey())

					if err == LastError {
						log.Debugf("%s Done", sess.SessionKey())
					}

				} else if newAck != nil {
					log.Debugln(newAck)
					go func() { prtCh <- newAck.Encode() }()
				}
			}
		}
	}
}
