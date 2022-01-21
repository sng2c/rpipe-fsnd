package jobqueue

import (
	"errors"
	"fmt"
	"github.com/juju/fslock"
	"github.com/radovskyb/watcher"
	"log"
	"os"
	"path"
	"regexp"
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
	BaseDir string
	State   JobState
	JobName string
	Payload string
}

func (job Job) Path() string {
	return path.Join(job.BaseDir, string(job.State), job.JobName)
}

func loadJob(jobPath string) (Job, error) {
	job := Job{}
	content, err := os.ReadFile(jobPath)
	if err != nil {
		log.Println("jobfile cannot be read")
		return Job{}, err
	}
	jobName := path.Base(path.Dir(jobPath))
	state := path.Base(path.Dir(path.Dir(jobPath)))
	baseDir := path.Dir(path.Dir(path.Dir(jobPath)))
	job.JobName = jobName
	job.BaseDir = baseDir
	if job.State, err = ToJobState(state); err != nil {
		return job, err
	}
	job.Payload = string(content)
	return job, nil
}

func (job Job) MoveJob(newState JobState) (Job, error) {
	jobOldPath := job.Path()
	lock := fslock.New(path.Join(job.BaseDir, string(job.State)))
	lockErr := lock.TryLock()
	if lockErr != nil {
		fmt.Println("falied to acquire lock > " + lockErr.Error())
		return job, lockErr
	}
	defer func() {
		// release the lock
		err := lock.Unlock()
		if err != nil {
			fmt.Println("falied to unlock > " + err.Error())
		}
		log.Print("release the lock")
	}()

	log.Print("got the lock")

	job.State = newState
	err := os.Rename(jobOldPath, job.Path())
	if err != nil {
		return job, err
	}

	return job, nil
}

func StartJobQueue(jobQueueBase string) (<-chan Job, error) {
	// SENDER
	jobCh := make(chan Job)
	w := watcher.New()
	// Only notify rename and move events.
	os.MkdirAll(path.Join(jobQueueBase, string(StateReady)),0755)
	os.MkdirAll(path.Join(jobQueueBase, string(StateDoing)),0755)
	os.MkdirAll(path.Join(jobQueueBase, string(StateFailed)),0755)
	os.MkdirAll(path.Join(jobQueueBase, string(StateDone)),0755)
	w.FilterOps(watcher.Move, watcher.Create, watcher.Write)

	r := regexp.MustCompile("^job.txt$")
	w.AddFilterHook(watcher.RegexFilterHook(r, false))
	if err := w.AddRecursive(path.Join(jobQueueBase, string(StateReady))); err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event := <-w.Event:
				fmt.Println(event) // Print the event's info.
				job, err := loadJob(event.Path)
				if err != nil {
					log.Println(err)
					continue
				}
				jobCh <- job
			case err := <-w.Error:
				log.Println(err)
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