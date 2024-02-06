package asynq

import (
	"context"
	"encoding/json"
	"purrproof/smartcrawl/app"

	"github.com/hibiken/asynq"
	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
)

var _ app.IJobQueue = (*JobQueue)(nil)

type JobQueue struct {
	WorkersNum uint
	client     *asynq.Client
	config     *app.QueueConfig
	jobHandler func(payload []byte) ([]app.IJob, error)
}

//error is impossible here, but it could be possible in other library, so lets keep error in return
func NewJobQueue(conf *app.QueueConfig, jobHandler func(payload []byte) ([]app.IJob, error)) (*JobQueue, error) {
	jobQueue := &JobQueue{
		WorkersNum: uint(1),
		config:     conf,
		jobHandler: jobHandler,
	}
	logrus.WithFields(logrus.Fields{}).Debug("job queue initialized")
	return jobQueue, nil
}

func (q *JobQueue) setClient() {
	client := asynq.NewClient(asynq.RedisClientOpt{
		Addr:     q.config.Addr,
		Password: q.config.Password,
	})
	q.client = client
}

func (q *JobQueue) Add(job app.IJob, params ...string) (*app.JobInfo, error) {
	//queueName = params[0] if set

	if q.client == nil {
		q.setClient()
	}

	payload, err := json.Marshal(job)
	if err != nil {
		return nil, errors.Annotate(err, "can't unmarshal job")
	}

	//https://github.com/hibiken/asynq#quickstart
	//https://github.com/hibiken/asynq/wiki/Queue-Priority
	//task options can be passed to NewTask, which can be overridden at enqueue time.
	//asynq.Queue("critical") config
	//asynq.MaxRetry(5) config.Job.MaxRetry
	//asynq.Timeout(20*time.Minute) config.Job.TimeoutSec
	//other options
	//asynq has no priority for single job, only for queue

	var queueName string
	if len(params) == 0 {
		//by default queue name is job name
		queueName = job.GetDefaultQueueName()
	} else {
		queueName = params[0]
	}
	task := asynq.NewTask(job.GetName(), payload, asynq.Queue(queueName))

	info, err := q.client.Enqueue(task)
	if err != nil {
		return nil, errors.Annotate(err, "can't enqueue job")
	}

	jobInfo := &app.JobInfo{
		Id:    info.ID,
		Queue: info.Queue,
	}
	return jobInfo, nil
}

func (q *JobQueue) Close() error {
	if q.client != nil {
		err := q.client.Close()
		if err != nil {
			return errors.Annotate(err, "can't close job queue")
		}
	}
	logrus.Info("queue closed")
	return nil
}

func (q *JobQueue) Process(qname string, workersNum uint) error {
	if workersNum == 0 {
		workersNum = 1
	}
	q.WorkersNum = workersNum
	srv := asynq.NewServer(
		asynq.RedisClientOpt{
			Addr:     q.config.Addr,
			Password: q.config.Password,
		},
		asynq.Config{
			// Specify how many concurrent workers to use
			Concurrency: int(q.WorkersNum),
			// Optionally specify multiple queues with different
			Queues: map[string]int{
				/*"critical": 6,
				"default":  3,
				"low":      1,*/
				qname: 10,
			},
			// See the godoc for other configuration options
		},
	)

	// mux maps a type to a handler
	mux := asynq.NewServeMux()
	//!!! We add each type of task to its separate queue designated for that type only.
	// Accordingly, one queue - one type of task.
	//https://github.com/hibiken/asynq/blob/95c90a5cb822877a9465cd3203235a262359096f/servemux.go#L134
	// Patterns are also accepted, but they work based on hasprefix, comparing the start of the string.
	// typ := "*" the asterisk won't work.
	// All tasks starting with job: will be processed, i.e., basically all tasks.
	// But since we have only one type of task in a queue, this is not a problem.	mux.HandleFunc("job:", q.handleTask)

	//mux.Handle(tasks.TypeImageResize, tasks.NewImageProcessor())
	// ...register other handlers...

	logrus.WithFields(logrus.Fields{
		"queue_name": qname,
	}).Info("start listening")

	if err := srv.Run(mux); err != nil {
		return errors.Annotate(err, "can't run job queue server")
	}

	logrus.WithFields(logrus.Fields{
		"queue_name": qname,
	}).Info("stop listening")

	return nil
}

func (q *JobQueue) handleTask(ctx context.Context, t *asynq.Task) error {
	//options: https://github.com/hibiken/asynq/blob/9116c096ecf3e8493f9997d2b906fa847b0d1a2c/client.go#L67
	//fmt.Println(t.Type())
	//fmt.Println(t.Payload())
	newJobs, err := q.jobHandler(t.Payload())
	for _, job := range newJobs {
		//add job to queue
		info, err := q.Add(job)
		if err != nil {
			return errors.Annotate(err, "can't add job to queue")
		}
		logrus.WithFields(logrus.Fields{
			"id":    info.Id,
			"queue": info.Queue,
		}).Info("job queued")
	}
	return err
}
