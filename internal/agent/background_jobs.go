package agent

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const maxBackgroundJobs = 64

type backgroundJob struct {
	id      int
	pid     int
	command string
	cwd     string
	started time.Time
	done    bool
	exitErr error
	output  string
	waiters chan struct{}
}

// jobManager tracks background run_command processes for a registry.
type jobManager struct {
	mu     sync.Mutex
	nextID int
	jobs   map[int]*backgroundJob
}

func newJobManager() *jobManager {
	return &jobManager{jobs: make(map[int]*backgroundJob)}
}

func (jm *jobManager) start(cmd *exec.Cmd, command, cwd string) (int, int, error) {
	jm.mu.Lock()
	defer jm.mu.Unlock()

	active := 0
	for _, j := range jm.jobs {
		if !j.done {
			active++
		}
	}
	if active >= maxBackgroundJobs {
		return 0, 0, fmt.Errorf("too many background jobs (max %d)", maxBackgroundJobs)
	}

	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	if err := cmd.Start(); err != nil {
		return 0, 0, err
	}

	id := jm.nextID
	jm.nextID++
	job := &backgroundJob{
		id:      id,
		pid:     cmd.Process.Pid,
		command: command,
		cwd:     cwd,
		started: time.Now(),
		waiters: make(chan struct{}),
	}
	jm.jobs[id] = job

	go func() {
		job.exitErr = cmd.Wait()
		job.output = buf.String()
		job.done = true
		close(job.waiters)
	}()

	return id, job.pid, nil
}

func (jm *jobManager) wait(id int, timeoutSec int) (string, error) {
	jm.mu.Lock()
	job, ok := jm.jobs[id]
	jm.mu.Unlock()
	if !ok {
		return "wait_job: job not found: " + fmt.Sprint(id), nil
	}

	if !job.done {
		if timeoutSec > 0 {
			select {
			case <-job.waiters:
			case <-time.After(time.Duration(timeoutSec) * time.Second):
				return fmt.Sprintf("wait_job: job %d still running (pid %d, started %s)",
					job.id, job.pid, job.started.Format(time.RFC3339)), nil
			}
		} else {
			<-job.waiters
		}
	}

	return formatJobResult(job), nil
}

func (jm *jobManager) list() string {
	jm.mu.Lock()
	defer jm.mu.Unlock()
	if len(jm.jobs) == 0 {
		return "list_background_jobs: no jobs"
	}
	var b bytes.Buffer
	fmt.Fprintf(&b, "list_background_jobs: %d job(s)\n", len(jm.jobs))
	for _, job := range jm.jobs {
		status := "running"
		if job.done {
			if job.exitErr != nil {
				status = "exited with error"
			} else {
				status = "completed"
			}
		}
		fmt.Fprintf(&b, "- job %d pid=%d status=%s command=%q cwd=%q started=%s\n",
			job.id, job.pid, status, job.command, job.cwd, job.started.Format(time.RFC3339))
	}
	return b.String()
}

func formatJobResult(job *backgroundJob) string {
	var b bytes.Buffer
	if job.exitErr != nil {
		fmt.Fprintf(&b, "wait_job: job %d exited with error: %v\n", job.id, job.exitErr)
	} else {
		fmt.Fprintf(&b, "wait_job: job %d completed\n", job.id)
	}
	if out := job.output; out != "" {
		b.WriteString(out)
		if !strings.HasSuffix(out, "\n") {
			b.WriteByte('\n')
		}
	}
	return b.String()
}
