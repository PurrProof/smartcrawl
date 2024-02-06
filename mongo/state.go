package mongo

import (
	"context"
	"time"

	"purrproof/smartcrawl/app"

	"github.com/juju/errors"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ app.IAppStateStore = (*AppStateStore)(nil)

type AppStateStore struct {
	client   *mongo.Client
	config   *app.StorageConfig
	collName string
}

//it's possible to move this into config.Storage, but I don't want to make config too big
//it could be done later in case of need
const appStateCollName = "state"

func NewAppStateStore(conf *app.StorageConfig) (*AppStateStore, error) {

	clientOptions := options.Client().ApplyURI(conf.Uri)

	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, errors.Annotate(err, "can't connect to mongo")
	}

	// check connection
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, errors.Annotate(err, "can't connect to mongo")
	}

	logrus.WithFields(logrus.Fields{}).Info("AppStateStore initialized")

	return &AppStateStore{
		client:   client,
		config:   conf,
		collName: appStateCollName,
	}, nil
}

func (s *AppStateStore) Save(info *app.AppState) error {
	coll := s.client.Database(s.config.DbName).Collection(s.collName)
	info.UpdatedAt = time.Now()
	filter := bson.D{}
	update := bson.D{{"$set", info}}
	opts := options.Update().SetUpsert(true)
	_, err := coll.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		return errors.Annotate(err, "can't update record")
	}
	return nil
}

func (s *AppStateStore) Get() (*app.AppState, error) {
	coll := s.client.Database(s.config.DbName).Collection(s.collName)
	var result *app.AppState

	filter := bson.D{}
	err := coll.FindOne(context.TODO(), filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			//state has not been saved yet, create new struct and return
			state := &app.AppState{
				LatestQueuedContainer: nil,
			}
			return state, nil
		}
		return nil, errors.Annotate(err, "can't get app state")
	}
	return result, nil
}

func (s *AppStateStore) Close() error {
	if s.client == nil {
		return nil
	}
	err := s.client.Disconnect(context.TODO())
	if err != nil {
		return errors.Annotate(err, "can't disconnect mongo client")
	}
	logrus.Info("app state store closed")
	return nil
}
