package factory

import (
	"encoding/json"
	"purrproof/smartcrawl/app"
	"purrproof/smartcrawl/app/job"

	"github.com/juju/errors"
)

func (f *Factory) UnmarshalJob(payload []byte) (app.IJob, error) {
	test := struct{ Name string }{}
	err := json.Unmarshal(payload, &test)
	if err != nil {
		return nil, errors.Annotate(err, "can't unmarshal job")
	} else if test.Name == "" {
		return nil, errors.New("unmarshaled job has empty 'name' field")
	}

	var jobres app.IJob
	switch test.Name {
	case job.JobTypeContainerProcess:
		jobres = &job.JobContainerProcess{}
	case job.JobTypePropertySet:
		jobres = &job.JobPropertySet{}
	default:
		return nil, errors.Errorf("can't restore job, unknown name: %s", test.Name)
	}

	err = json.Unmarshal(payload, &jobres)
	if err != nil {
		return nil, errors.Annotate(err, "can't unmarshal job")
	}

	//provider
	provider, err := f.GetProviderByKey(jobres.GetProviderKey())
	if err != nil {
		return nil, errors.Annotatef(err, "can't get item provider for job name=%s", job.JobTypeContainerProcess)
	}
	jobres.SetItemProvider(provider)
	//repository
	repository, err := f.GetItemRepository()
	if err != nil {
		return nil, errors.Annotatef(err, "can't get item repository for job name=%s", job.JobTypeContainerProcess)
	}
	jobres.SetItemRepository(repository)
	return jobres, nil
}

func (f *Factory) HandleJobPayload(payload []byte) ([]app.IJob, error) {
	job, err := f.UnmarshalJob(payload)
	if err != nil {
		return nil, errors.Annotate(err, "can't unmarshal job")
	}
	newJobs, err := job.Execute()
	if err != nil {
		return newJobs, errors.Annotate(err, "can't execute job")
	}
	return newJobs, nil
}
