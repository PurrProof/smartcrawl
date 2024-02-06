package mongo

import (
	"context"
	"time"

	"purrproof/smartcrawl/app"

	"github.com/juju/errors"
	"github.com/oleiade/reflections"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ app.IItemRepository = (*ItemRepository)(nil)

type ItemRepository struct {
	client   *mongo.Client
	config   *app.StorageConfig
	collName string
	coll     *mongo.Collection
}

//it's possible to move this into config.Storage, but I don't want to make config too big
//it could be done later in case of need
const itemCollName = "item"

func NewItemRepository(conf *app.StorageConfig) (*ItemRepository, error) {
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

	logrus.WithFields(logrus.Fields{}).Debug("item repository initialized")

	coll := client.Database(conf.DbName).Collection(itemCollName)

	return &ItemRepository{
		client:   client,
		config:   conf,
		collName: itemCollName,
		coll:     coll,
	}, nil
}

func (s *ItemRepository) Save(item app.IItem) error {
	item.SetBaseField("UpdatedAt", time.Now())
	filterId := item.GetId()
	filter, err := bson.Marshal(filterId)
	if err != nil {
		return errors.Annotate(err, "can't marshal item filter")
	}

	update := bson.D{{"$set", item}}
	opts := options.Update().SetUpsert(true)
	_, err = s.coll.UpdateOne(context.TODO(), filter, update, opts)
	if err != nil {
		return errors.Annotate(err, "can't save item")
	}
	return nil
}

func (s *ItemRepository) Update(item app.IItem, fieldNames []string) error {
	item.SetBaseField("UpdatedAt", time.Now())
	filterId := item.GetId()
	filter, err := bson.Marshal(filterId)
	if err != nil {
		return errors.Annotate(err, "can't marshal item filter")
	}

	fields := bson.M{}
	for _, fname := range fieldNames {
		fvalue, err := reflections.GetField(item, fname)
		if err != nil {
			return errors.Annotatef(err, "can't get item field, fname=%s", fname)
		}
		ftag, err := reflections.GetFieldTag(item, fname, "bson")
		if err != nil {
			return errors.Annotatef(err, "can't get item field, fname=%s", fname)
		}
		fields[ftag] = fvalue
	}

	update := bson.M{
		"$set": fields,
	}

	_, err = s.coll.UpdateOne(context.TODO(), filter, update)
	if err != nil {
		return errors.Annotate(err, "can't update item")
	}
	return nil
}

func (s *ItemRepository) Get(item app.IItem) (app.IItem, error) {
	filterId := item.GetId()
	filter, err := bson.Marshal(filterId)
	if err != nil {
		return nil, errors.Annotate(err, "can't marshal item filter")
	}

	//if .Decode(&item), there is error "no decoder found for app.IItem"
	err = s.coll.FindOne(context.TODO(), filter).Decode(item)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logrus.WithFields(logrus.Fields{
				"item_id": item.GetId().String(),
			}).Info("item not found")
			return nil, nil
		}
		return nil, errors.Annotate(err, "can't get item from db")
	}
	return item, nil
}

func (s *ItemRepository) GetAllWithoutProperty(provider app.IItemProvider, propName string, limit uint) ([]app.IItem, error) {
	testItem := provider.NewItem("")
	dbField, err := reflections.GetFieldTag(testItem, propName, "bson")
	if err != nil {
		return nil, errors.Annotatef(err, "can't get item field, fname=%s", propName)
	}

	filter := bson.M(testItem.GetProviderFilter())
	filter[dbField] = bson.M{"$exists": false}

	findOptions := options.Find()
	findOptions.SetLimit(int64(limit))

	cursor, err := s.coll.Find(context.TODO(), filter, findOptions)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			logrus.WithFields(logrus.Fields{
				"property_name": propName,
			}).Info("no items found")
			return nil, nil
		}
		return nil, errors.Annotate(err, "can't get items from db")
	}

	//https://kb.objectrocket.com/mongo-db/how-to-get-mongodb-documents-using-golang-446#use+golang%5C%27s+context+package+to+manage+the+mongodb+api+requests
	ctx, _ := context.WithTimeout(context.Background(), 15*time.Second)
	result := make([]app.IItem, 0)
	for cursor.Next(ctx) {
		item := provider.NewItem("")
		err := cursor.Decode(item)
		if err != nil {
			return nil, errors.Trace(err)
		}
		result = append(result, item)
	}
	defer cursor.Close(ctx)
	return result, nil
}

func (s *ItemRepository) Close() error {
	if s.client == nil {
		return nil
	}
	err := s.client.Disconnect(context.TODO())
	if err != nil {
		return errors.Annotate(err, "can't disconnect mongo client")
	}
	logrus.Info("repository closed")
	return nil
}
