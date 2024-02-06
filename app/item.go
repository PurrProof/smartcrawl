package app

import (
	"reflect"
	"time"

	"github.com/juju/errors"
)

type Item struct {
	//https://stackoverflow.com/questions/23164417/mongodb-and-composite-primary-keys
	//I decided don't do composite index for now
	//each item will have following 3 indexed fields instead
	//may be I'll change that in next version
	ProvName            string
	ProvBranch          string
	Id                  string
	UpdatedAt           time.Time
	realtimeAutosetters map[string]func() error
	delayedAutosetters  map[string]func() error
}

type ItemId struct {
	ProvName   string `bson:"provname"`
	ProvBranch string `bson:"provbranch"`
	Id         string `bson:"id"`
}

func (iid *ItemId) String() string {
	return iid.ProvName + "_" + iid.ProvBranch + "_" + iid.Id
}

var _ IItem = (*Item)(nil)

func NewItem(provName, provBranch, id string) *Item {
	item := &Item{
		ProvName:   provName,
		ProvBranch: provBranch,
		Id:         id,
		//UpdatedAt:           nil,
		realtimeAutosetters: make(map[string]func() error, 0),
		delayedAutosetters:  make(map[string]func() error, 0),
	}
	return item
}

func (e *Item) GetId() *ItemId {
	return &ItemId{
		ProvName:   e.ProvName,
		ProvBranch: e.ProvBranch,
		Id:         e.Id,
	}
}

func (e *Item) GetProviderFilter() map[string]interface{} {
	result := make(map[string]interface{}, 0)
	result["provname"] = e.ProvName
	result["provbranch"] = e.ProvBranch
	return result
}

func (e *Item) SetBaseField(fieldName string, fieldValue interface{}) error {

	_, found := reflect.TypeOf(e).Elem().FieldByName(fieldName)
	if !found {
		return errors.Errorf("field not found, name=%s", fieldName)
	}

	//https: //stackoverflow.com/questions/57146301/using-reflect-how-to-set-value-to-a-struct-field-pointer
	reflect.ValueOf(e).Elem().FieldByName(fieldName).Set(reflect.ValueOf(fieldValue))

	return nil
}

func (e *Item) RegisterRealtimeAutosetter(name string, p func() error) {
	e.realtimeAutosetters[name] = p
}

func (e *Item) RegisterDelayedAutosetter(name string, p func() error) {
	e.delayedAutosetters[name] = p
}

func (e *Item) GetRealtimeAutosetters() map[string]func() error {
	return e.realtimeAutosetters
}

func (e *Item) GetDelayedAutosetters() map[string]func() error {
	return e.delayedAutosetters
}

/*
just call autosetter by its name
autosetter's type doesn't matter in this method
*/
func (e *Item) CallAutosetter(name string) error {
	if function, found := e.realtimeAutosetters[name]; found {
		return function()
	} else if function, found := e.delayedAutosetters[name]; found {
		return function()
	}
	return errors.Errorf("autosetter %s not found", name)
}

func (e *Item) CallRealtimeAutosetter(name string) error {
	if function, found := e.realtimeAutosetters[name]; found {
		return function()
	}
	return errors.Errorf("autosetter %s not found", name)
}

func (e *Item) CallAllRealtimeAutosetters() {
	for _, function := range e.realtimeAutosetters {
		function()
	}
}

func (e *Item) RegisterAutosetters() error {
	panic("Method of base struct must be overwritten")
	return nil
}

func (e *Item) HasAutosetField(name string) bool {
	if _, found := e.realtimeAutosetters[name]; found {
		return true
	} else if _, found := e.delayedAutosetters[name]; found {
		return true
	}
	return false
}
