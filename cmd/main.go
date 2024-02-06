package main

import (
	"fmt"
	"os"

	"purrproof/smartcrawl/app"
	"purrproof/smartcrawl/app/job"
	factory_pkg "purrproof/smartcrawl/factory"

	"github.com/joho/godotenv"
	"github.com/juju/errors"
	"github.com/oleiade/reflections"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v3"
)

const (
	flagEnv          string = "env"
	flagProvider     string = "provider"
	flagLogLevel     string = "log-level"
	flagLimit        string = "limit"
	flagQueue        string = "queue"
	flagContainer    string = "container"
	flagContainerReq string = "container"
	flagProperty     string = "property"
	flagItem         string = "item"
)

type CliFlags struct {
	Env          cli.Flag
	Provider     cli.Flag
	LogLevel     cli.Flag
	Limit        cli.Flag
	Queue        cli.Flag
	Container    cli.Flag
	ContainerReq cli.Flag
	Property     cli.Flag
	Item         cli.Flag
}

var cliFlags = CliFlags{
	//global flags
	Env: &cli.StringFlag{
		Name:       flagEnv,
		Persistent: true,
		Value:      "local",
		Usage:      "environment",
		Required:   false,
	},
	Provider: &cli.StringFlag{
		Name:       flagProvider,
		Persistent: true,
		Usage:      "provider",
		Required:   true,
	},
	LogLevel: &cli.StringFlag{
		Name:       flagLogLevel,
		Persistent: true,
		Value:      "debug",
		Usage:      "log level",
		Required:   false,
	},
	//other flags
	Limit: &cli.UintFlag{
		Name:     flagLimit,
		Value:    1,
		Usage:    "number of objects (workers, containers), depending from context",
		Required: false,
	},
	Queue: &cli.StringFlag{
		Name:     flagQueue,
		Value:    "",
		Usage:    "queue name",
		Required: true,
	},
	Container: &cli.StringSliceFlag{
		Name:     flagContainer,
		Value:    []string{},
		Usage:    "container id (use multiple times for compound id)",
		Required: false,
	},
	ContainerReq: &cli.StringSliceFlag{
		Name:     flagContainer,
		Value:    []string{},
		Usage:    "container id (use multiple times for compound id)",
		Required: true,
	},
	Property: &cli.StringFlag{
		Name:     flagProperty,
		Value:    "",
		Usage:    "property name",
		Required: true,
	},
	Item: &cli.StringFlag{
		Name:     flagItem,
		Value:    "",
		Usage:    "item id",
		Required: true,
	},
}

var appConfig *app.AppConfig
var factory *factory_pkg.Factory
var providerKey string
var provider app.IItemProvider

func main() {
	cliapp := &cli.App{
		Name: "crawler",
		Flags: []cli.Flag{
			cliFlags.Env,
			cliFlags.Provider,
			cliFlags.LogLevel,
		},
		Commands: []*cli.Command{
			CmdExecContainerProcess(),
			CmdExecPropertySet(),
			CmdQueueContainerProcess(),
			CmdQueuePropertyAdd(),
			CmdWorker(),
		},
		Before: func(c *cli.Context) error {
			//load environment variables from file
			env := c.String(flagEnv)
			fname := fmt.Sprintf(".env.%s", env)
			if err := godotenv.Load(fname); err != nil {
				return errors.Annotatef(err, "can't load env file, name=%s", fname)
			}

			//load app config
			var err error
			appConfig, err = app.NewConfig()
			if err != nil {
				return errors.Annotate(err, "can't initialize config")
			}

			//init factory
			factory = factory_pkg.NewFactory(appConfig)

			//init provider
			providerKey = c.String(flagProvider)
			provider, err = factory.GetProviderByKey(providerKey)
			if err != nil {
				return errors.Trace(err)
			}

			//init logger
			//not found a way to modify LogLevel flag default, so use if
			flaglev := c.String(flagLogLevel)
			level := appConfig.LogLevel
			if flaglev != "" {
				level = flaglev
			}

			parsed, err := logrus.ParseLevel(level)
			if err != nil {
				logrus.Fatal(err)
			}

			logrus.SetLevel(parsed)
			logrus.SetFormatter(&logrus.TextFormatter{
				FullTimestamp: true,
			})

			return nil
		},
	}
	if err := cliapp.Run(os.Args); err != nil {
		logrus.Fatal(errors.ErrorStack(err))
	}
	if factory != nil {
		defer factory.RunDeferred()
	}
}

func CmdExecContainerProcess() *cli.Command {
	return &cli.Command{
		Name:  "exec-container-process",
		Usage: "execute single job:container:process job (and subsequent jobs) for item id and property name",
		Flags: []cli.Flag{
			cliFlags.ContainerReq,
		},
		Action: func(c *cli.Context) error {

			//get container
			containerId := c.StringSlice(flagContainer)
			container := app.NewItemsContainer(containerId)

			//repository
			repository, err := factory.GetItemRepository()
			if err != nil {
				return errors.Trace(err)
			}

			//create job message
			thejob := job.NewMessageJobContainerProcess(providerKey, container)
			thejob.SetItemProvider(provider)
			thejob.SetItemRepository(repository)

			newJobs, err := thejob.Execute()
			if err != nil {
				return errors.Annotate(err, "can't execute job")
			}

			//execute returned property set jobs
			for _, jobPropertySet := range newJobs {
				jobPropertySet.SetItemProvider(provider)
				jobPropertySet.SetItemRepository(repository)
				//this job don't return any other jobs
				_, err := jobPropertySet.Execute()
				if err != nil {
					logrus.WithError(err).WithFields(logrus.Fields{
						"job": jobPropertySet,
					}).Warning("can't execute job")
					return errors.Annotate(err, "can't execute job")
				}
			}

			return nil
		},
	}
}

func CmdExecPropertySet() *cli.Command {

	return &cli.Command{
		Name:  "exec-property-set",
		Usage: "execute single job:property:set job for item id and property name",
		Flags: []cli.Flag{
			cliFlags.Item,
			cliFlags.Property,
		},
		Action: func(c *cli.Context) error {

			itemId := c.String(flagItem)
			item := provider.NewItem(itemId)

			//get property name
			propName := c.String(flagProperty)
			if !item.HasAutosetField(propName) {
				return errors.Errorf("not found property name=%s for provider=%s", propName, providerKey)
			}

			//repository
			repository, err := factory.GetItemRepository()
			if err != nil {
				return errors.Trace(err)
			}

			//create job
			thejob := job.NewMessageJobPropertySet(providerKey, item.GetId(), propName)
			thejob.SetItemProvider(provider)
			thejob.SetItemRepository(repository)

			//this job don't return any other jobs
			_, err = thejob.Execute()
			if err != nil {
				return errors.Annotate(err, "can't execute job")
			}

			return nil
		},
	}
}

func CmdQueueContainerProcess() *cli.Command {

	return &cli.Command{
		Name:  "queue-container-process",
		Usage: "Get portion of containers and queue them for processing",
		Flags: []cli.Flag{
			cliFlags.Limit,
			cliFlags.Container,
		},
		Action: func(c *cli.Context) error {

			//init app state store
			stateStore, err := factory.GetAppStateStore()
			if err != nil {
				return errors.Trace(err)
			}

			//get app state
			state, err := stateStore.Get()
			if err != nil {
				return errors.Annotate(err, "can't get app state store")
			}

			//get start container
			saveState := true
			containerId := c.StringSlice(flagContainer)
			var startContainer *app.ItemsContainer
			if len(containerId) > 0 {
				//don't save state if container specified manually
				//may be change this later
				saveState = false
				startContainer = app.NewItemsContainer(containerId)
			} else {
				startContainer = state.LatestQueuedContainer
			}

			//get container limit flag
			containersNum := c.Uint(flagLimit)

			//get containers list
			list, err := provider.GetContainersList(containersNum, startContainer)
			if err != nil {
				return errors.Annotate(err, "can't get containers list")
			}

			//init queue
			jobQueue, err := factory.GetJobQueue()
			if err != nil {
				return errors.Trace(err)
			}

			//add jobs to queue in cycle
			for _, container := range list {
				jobIn := job.NewMessageJobContainerProcess(providerKey, container)
				//add job to queue
				info, err := jobQueue.Add(jobIn)
				if err != nil {
					return errors.Annotate(err, "can't add job to queue")
				}
				logrus.WithFields(logrus.Fields{
					"id":        info.Id,
					"queue":     info.Queue,
					"container": container,
				}).Info("job queued")

				if saveState {
					state.LatestQueuedContainer = container
					stateStore.Save(state)
				}
				if err != nil {
					return errors.Annotate(err, "can't save application state")
				}
			}

			return nil
		},
	}
}

func CmdQueuePropertyAdd() *cli.Command {

	return &cli.Command{
		Name:  "queue-property-add",
		Usage: "queue many job:property:set jobs for items without that property",
		Flags: []cli.Flag{
			cliFlags.Property,
			cliFlags.Limit,
		},
		Action: func(c *cli.Context) error {

			//get items limit
			limit := c.Uint(flagLimit)
			if limit == 0 {
				return errors.New("Limit must be greater than 0")
			}

			//get property name
			propName := c.String(flagProperty)
			testItem := provider.NewItem("test")
			if !testItem.HasAutosetField(propName) {
				return errors.Errorf("not found property name=%s for provider=%s", propName, providerKey)
			}

			//repository
			repository, err := factory.GetItemRepository()
			if err != nil {
				return errors.Trace(err)
			}

			//get items without property
			items, err := repository.GetAllWithoutProperty(provider, propName, limit)
			if err != nil {
				return errors.Trace(err)
			}
			logrus.WithFields(logrus.Fields{"number": len(items)}).Info("got items")

			//init queue
			jobQueue, err := factory.GetJobQueue()
			if err != nil {
				return errors.Trace(err)
			}

			i := 0
			for _, item := range items {
				//create job message
				jobmsg := job.NewMessageJobPropertySet(providerKey, item.GetId(), propName)

				//add job to queue
				info, err := jobQueue.Add(jobmsg)
				if err != nil {
					return errors.Annotate(err, "can't add job to queue")
				}

				//mark item field as processed
				reflections.SetField(item, propName, "")
				err = repository.Update(item, []string{propName})
				if err != nil {
					return errors.Annotatef(err, "can't save item: %s", item)
				}
				i++
				logrus.WithFields(logrus.Fields{
					"item_id":       item.GetId(),
					"property_name": propName,
					"job_id":        info.Id,
					"job_queue":     info.Queue,
					"i":             i,
				}).Debug("job queued, item saved")
			}

			logrus.WithFields(logrus.Fields{
				"number": len(items),
			}).Info("items queued")

			return nil
		},
	}
}

func CmdWorker() *cli.Command {

	return &cli.Command{
		Name:  "worker",
		Usage: "start workers",
		Flags: []cli.Flag{
			cliFlags.Queue,
			cliFlags.Limit,
		},
		Action: func(c *cli.Context) error {

			jobQueue, err := factory.GetJobQueue()
			if err != nil {
				return errors.Trace(err)
			}

			//get container limit flag
			workersNum := c.Uint(flagLimit)

			//get queue name flag
			qname := c.String(flagQueue)

			jobQueue.Process(qname, workersNum)

			return nil

		},
	}
}
