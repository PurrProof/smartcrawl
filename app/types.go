package app

import "time"

type IItemProvider interface {
	NewItem(id string) IItem
	/*
		Returns list of containers, for example:
		1) list of blocks
		2) list of pages in software catalog
	*/
	GetContainersList(number uint, startAfter *ItemsContainer) ([]*ItemsContainer, error)

	/*
		Returns items array from container.
		For example,
		1) deployed contracts from block
		2) pattern matching html pieces from page in case of grabbing
		3) applications/libraries from software category
	*/
	FetchContainerItems(container *ItemsContainer) ([]IItem, error)
	PrepareItemsArray(limit uint) []IItem
	Close() error
}

type IItem interface {
	HasAutosetField(name string) bool
	RegisterAutosetters() error
	RegisterRealtimeAutosetter(name string, p func() error)
	RegisterDelayedAutosetter(name string, p func() error)
	GetRealtimeAutosetters() map[string]func() error
	GetDelayedAutosetters() map[string]func() error
	CallRealtimeAutosetter(name string) error
	CallAllRealtimeAutosetters()
	CallAutosetter(name string) error
	SetBaseField(fieldName string, fieldValue interface{}) error
	GetId() *ItemId
	GetProviderFilter() map[string]interface{}
}

type IItemRepository interface {
	Get(item IItem) (IItem, error)
	GetAllWithoutProperty(provider IItemProvider, propName string, limit uint) ([]IItem, error)
	Save(item IItem) error
	Update(item IItem, fieldNames []string) error
	Close() error
}

type IJob interface {
	GetName() string
	GetDefaultQueueName() string
	GetProviderKey() string
	SetItemRepository(repository IItemRepository)
	SetItemProvider(provider IItemProvider)
	Execute() ([]IJob, error)
}

type IJobQueue interface {
	Add(job IJob, params ...string) (*JobInfo, error)
	Close() error
	Process(queue string, workersNum uint) error
}

type JobInfo struct {
	Id    string
	Queue string
}

type AppState struct {
	LatestQueuedContainer *ItemsContainer
	UpdatedAt             time.Time
}

type IAppStateStore interface {
	Get() (*AppState, error)
	Save(info *AppState) error
	Close() error
}
