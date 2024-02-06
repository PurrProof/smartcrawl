package job

import (
	"purrproof/smartcrawl/app"

	"github.com/juju/errors"

	"github.com/sirupsen/logrus"
)

const JobTypePropertySet = "job:property:set"

var _ app.IJob = (*JobPropertySet)(nil)

type JobPropertySet struct {
	*app.Job
	ItemId       *app.ItemId
	PropertyName string
}

/*
it isn't real job, because we not set dependencies here
it's just job message to put into queue
*/
func NewMessageJobPropertySet(provKey string, itemId *app.ItemId, propName string) *JobPropertySet {
	return &JobPropertySet{
		Job: &app.Job{
			Name:        JobTypePropertySet,
			ProviderKey: provKey,
		},
		ItemId:       itemId,
		PropertyName: propName,
	}
}

func (j *JobPropertySet) Execute() ([]app.IJob, error) {

	if j.ProviderKey == "" {
		return nil, errors.Errorf("provider key is not defined, job=%s", j.Name)
	} else if j.ItemProvider == nil {
		return nil, errors.Errorf("provider is not defined, job=%s", j.Name)
	} else if j.ItemRepository == nil {
		return nil, errors.Errorf("repository is not defined, job=%s", j.Name)
	} else if j.ItemId == nil {
		return nil, errors.Errorf("item id is not defined, job=%s", j.Name)
	} else if j.PropertyName == "" {
		return nil, errors.Errorf("property name is not defined, job=%s", j.Name)
	}

	//create empty item with correct item id
	emptyItem := j.ItemProvider.NewItem(j.ItemId.Id)
	//get item from repository
	item, err := j.ItemRepository.Get(emptyItem)
	if err != nil {
		return nil, errors.Annotatef(err, "can't get item from repository, item id: %s", j.ItemId.String())
	} else if item == nil {
		return nil, errors.Annotatef(err, "item not found in repository, item id: %s", j.ItemId.String())
	}

	err = item.CallAutosetter(j.PropertyName)
	if err != nil {
		return nil, errors.Annotatef(err, "can't autoset property name=%s", j.PropertyName)
	}

	logrus.WithFields(logrus.Fields{
		"item_id":       j.ItemId.String(),
		"property_name": j.PropertyName,
	}).Info("property set successfully")

	//update item
	err = j.ItemRepository.Update(item, []string{j.PropertyName})
	if err != nil {
		return nil, errors.Annotatef(err, "can't save item: %s", item)
	}
	logrus.WithFields(logrus.Fields{"item_id": item.GetId()}).Debug("item saved")

	return nil, nil

}

func (j *JobPropertySet) GetDefaultQueueName() string {
	return JobTypePropertySet + ":" + j.PropertyName
}
