package jobqueue

import (
	"errors"
	"github.com/juju/fslock"
	"github.com/radovskyb/watcher"
	log "github.com/sirupsen/logrus"
	"os"
	"path"
	"regexp"
	"strings"
	"time"
)

type JobState string

const (
	StateReady  = JobState("ready")
	StateDoing  = JobState("doing")
	StateDone   = JobState("done")
	StateFailed = JobState("failed")
)

func ToJobState(name string) (JobState, error) {
	state := JobState(name)
	switch state {
	case StateReady:
	case StateDoing:
	case StateDone:
	case StateFailed:
	default:
		return JobState(""), errors.New("Not defined State " + name)
	}
	return state, nil
}

type Job struct {
	BaseDir  string
	State    JobState
	JobName  string
	FileName string
	Payload  []byte
}

func (job *Job) FilePath() string {
	return path.Join(job.BaseDir, string(job.State), job.FileName)
}
func (job *Job) JobPath() string {
	return path.Join(job.BaseDir, string(job.State), job.JobName)
}
func NewJobFromJobPath(jobPath string) (*Job, error) {
	job := &Job{}
	content, err := os.ReadFile(jobPath)
	if err != nil {
		log.Debugln("jobfile cannot be read")
		return nil, err
	}
	state := path.Base(path.Dir(jobPath))
	baseDir := path.Dir(path.Dir(jobPath))
	job.JobName = path.Base(jobPath)
	job.FileName = strings.TrimSuffix(job.JobName, ".JOB")
	job.BaseDir = baseDir
	if job.State, err = ToJobState(state); err != nil {
		return nil, err
	}
	job.Payload = content

	_, err = os.Stat(job.FilePath())
	if err != nil {
		return nil, err
	}

	return job, nil
}

func loadJob(jobPath string) (*Job, error) {
	return NewJobFromJobPath(jobPath)
}

func (job *Job) MoveJob(newState JobState) (*Job, error) {
	jobOldPath := path.Join(job.BaseDir, string(job.State), job.JobName)
	fileOldPath := path.Join(job.BaseDir, string(job.State), job.FileName)

	lock := fslock.New(path.Join(job.BaseDir, string(job.State)))
	lockErr := lock.TryLock()
	if lockErr != nil {
		log.Debugln("falied to acquire lock > " + lockErr.Error())
		return job, lockErr
	}
	defer func() {
		// release the lock
		err := lock.Unlock()
		if err != nil {
			log.Debugln("falied to unlock > " + err.Error())
		}
		log.Debug("release the lock")
	}()

	log.Debug("got the lock")

	job.State = newState
	err := os.Rename(jobOldPath, job.JobPath())
	if err != nil {
		return job, err
	}
	err = os.Rename(fileOldPath, job.FilePath())
	if err != nil {
		return job, err
	}

	return job, nil
}

func StartJobQueue(jobQueueBase string) (<-chan *Job, error) {
	// SENDER
	jobCh := make(chan *Job)
	w := watcher.New()
	// Only notify rename and move events.
	os.MkdirAll(path.Join(jobQueueBase, string(StateReady)), 0755)
	os.MkdirAll(path.Join(jobQueueBase, string(StateDoing)), 0755)
	os.MkdirAll(path.Join(jobQueueBase, string(StateFailed)), 0755)
	os.MkdirAll(path.Join(jobQueueBase, string(StateDone)), 0755)
	w.FilterOps(watcher.Move, watcher.Create, watcher.Write)

	r := regexp.MustCompile("\\.JOB$")
	w.AddFilterHook(watcher.RegexFilterHook(r, false))
	if err := w.AddRecursive(path.Join(jobQueueBase, string(StateReady))); err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event := <-w.Event:
				log.Debugln(event) // Print the event's info.
				job, err := loadJob(event.Path)
				if err != nil {
					log.Debugln(err)

					continue
				}
				jobCh <- job
			case err := <-w.Error:
				log.Debugln(err)
			case <-w.Closed:
				return
			}
		}
	}()

	go func() {
		// Start the watching process - it'll check for changes every 100ms.
		if err := w.Start(time.Millisecond * 100); err != nil {
			log.Fatalln(err)
		}
	}()
	return jobCh, nil
}
