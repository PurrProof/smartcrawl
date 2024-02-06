package factory

import (
	"purrproof/smartcrawl/app"
	"purrproof/smartcrawl/asynq"
	"purrproof/smartcrawl/mongo"

	"github.com/juju/errors"
)

type Factory struct {
	AppConfig      *app.AppConfig
	JobQueue       app.IJobQueue
	AppStateStore  app.IAppStateStore
	ItemRepository app.IItemRepository
	ItemProvider   map[string]app.IItemProvider
	deferred       []func() error
}

func NewFactory(appConfig *app.AppConfig) *Factory {
	factory := &Factory{
		AppConfig:    appConfig,
		ItemProvider: make(map[string]app.IItemProvider, 0),
		deferred:     make([]func() error, 0),
	}
	return factory
}

func (f *Factory) Defer(dfunc func() error) {
	f.deferred = append(f.deferred, dfunc)
}

func (f *Factory) RunDeferred() {
	for _, dfunc := range f.deferred {
		dfunc()
	}
}

func (f *Factory) GetJobQueue() (app.IJobQueue, error) {
	if f.JobQueue != nil {
		return f.JobQueue, nil
	}
	queue, err := asynq.NewJobQueue(f.AppConfig.Queue, f.HandleJobPayload)
	if err != nil {
		return nil, errors.Annotate(err, "can't initialize job queue")
	}
	f.JobQueue = queue
	f.Defer(f.JobQueue.Close)
	return queue, nil
}

func (f *Factory) GetItemRepository() (app.IItemRepository, error) {
	if f.ItemRepository != nil {
		return f.ItemRepository, nil
	}
	repository, err := mongo.NewItemRepository(f.AppConfig.Storage)
	if err != nil {
		return nil, errors.Annotate(err, "can't initialize item repository")
	}
	f.ItemRepository = repository
	f.Defer(f.ItemRepository.Close)
	return repository, nil
}

func (f *Factory) GetAppStateStore() (app.IAppStateStore, error) {
	if f.AppStateStore != nil {
		return f.AppStateStore, nil
	}
	stateStore, err := mongo.NewAppStateStore(f.AppConfig.Storage)
	if err != nil {
		return nil, errors.Annotate(err, "can't initialize app state store")
	}
	f.AppStateStore = stateStore
	f.Defer(f.AppStateStore.Close)
	return stateStore, nil
}
