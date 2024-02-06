package job

import (
	"purrproof/smartcrawl/app"

	"github.com/juju/errors"
	"github.com/oleiade/reflections"
	"github.com/sirupsen/logrus"
)

const JobTypeContainerProcess = "job:container:process"

var _ app.IJob = (*JobContainerProcess)(nil)

type JobContainerProcess struct {
	*app.Job
	Container *app.ItemsContainer
}

/*
it isn't real job, because we not set dependencies here
it's just job message to put into queue
*/
func NewMessageJobContainerProcess(provKey string, container *app.ItemsContainer) *JobContainerProcess {
	return &JobContainerProcess{
		Job: &app.Job{
			Name:        JobTypeContainerProcess,
			ProviderKey: provKey,
		},
		Container: container,
	}
}

func (j *JobContainerProcess) Execute() ([]app.IJob, error) {

	if j.ProviderKey == "" {
		return nil, errors.Errorf("provider key is not defined, job=%s", j.Name)
	} else if j.ItemProvider == nil {
		return nil, errors.Errorf("provider is not defined, job=%s", j.Name)
	} else if j.ItemRepository == nil {
		return nil, errors.Errorf("repository is not defined, job=%s", j.Name)
	} else if j.Container == nil {
		return nil, errors.Errorf("container is not defined, job=%s", j.Name)
	}

	items, err := j.ItemProvider.FetchContainerItems(j.Container)
	if err != nil {
		logrus.WithError(err).Error("can't fetch container items")
		return nil, errors.Trace(err)
	}

	logrus.WithFields(logrus.Fields{
		"container":   j.Container.String(),
		"items_found": len(items),
	}).Info("FetchContainerItems done")

	jobsOut := make([]app.IJob, 0)
	for _, item := range items {
		//realtime properties
		//we also could call item.CallAllRealtimeAutosetters() instead of whole cycle below
		props := item.GetRealtimeAutosetters()
		for name, _ /*autosetter*/ := range props {
			//we also could just call autosetter()
			err := item.CallRealtimeAutosetter(name)
			if err != nil {
				return nil, errors.Annotatef(err, "can't autoset property name=%s", name)
			}
			val, _ := reflections.GetField(item, name)
			logrus.WithFields(logrus.Fields{
				"property_name":  name,
				"property_value": val,
			}).Debug("property set")
		}

		logrus.WithFields(logrus.Fields{
			"count": len(props),
		}).Debug("all realtime properties set")

		//res, _ := json.Marshal(item)
		//fmt.Println(string(res))

		//save item
		err = j.ItemRepository.Save(item)
		if err != nil {
			return nil, errors.Annotatef(err, "can't save item: %s", item)
		}
		logrus.WithFields(logrus.Fields{"item_id": item.GetId()}).Debug("item saved")

		//delayed properties
		props = item.GetDelayedAutosetters()
		for propName := range props {
			//one property => one job
			thejob := NewMessageJobPropertySet(j.ProviderKey, item.GetId(), propName)
			jobsOut = append(jobsOut, thejob)
			logrus.WithFields(logrus.Fields{
				"job_name":         thejob.GetName(),
				"delayed_property": propName,
			}).Debug("job created")
		}

	}

	return jobsOut, nil
}
