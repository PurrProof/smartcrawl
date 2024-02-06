package app

type Job struct {
	Name           string
	ProviderKey    string
	ItemProvider   IItemProvider   `json:"-"` //we don't need to store this object in a job
	ItemRepository IItemRepository `json:"-"` //we don't need to store this object in a job
}

func (j *Job) GetName() string {
	return j.Name
}

func (j *Job) GetDefaultQueueName() string {
	return j.Name
}

func (j *Job) GetProviderKey() string {
	return j.ProviderKey
}

func (j *Job) SetItemRepository(repository IItemRepository) {
	j.ItemRepository = repository
}

func (j *Job) SetItemProvider(provider IItemProvider) {
	j.ItemProvider = provider
}
