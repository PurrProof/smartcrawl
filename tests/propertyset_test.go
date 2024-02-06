package tests

import (
	"encoding/json"
	"os"
	"purrproof/smartcrawl/app"
	"purrproof/smartcrawl/app/job"
	"purrproof/smartcrawl/mocks"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	factory_pkg "purrproof/smartcrawl/factory"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	os.Setenv("TESTS", "1")
}

func Test_PropertySet(t *testing.T) {

	providerKey := "zilmain"
	providerName := "Zilliqa"
	providerBranch := "1"
	iid := "73b8299f53e98fdf206ad3671e16407ed692a519"
	propertyName := "Name"

	appConfig, err := app.NewConfig("..")
	assert.Nil(t, err)

	//init factory
	factory := factory_pkg.NewFactory(appConfig)

	//init provider
	provider, err := factory.GetProviderByKey(providerKey)
	assert.Nil(t, err)
	assert.Equal(t, "*zilliqa.ZilliqaBlockchain", reflect.TypeOf(provider).String())

	//init repository
	repoMock := new(mocks.ItemRepositoryMock)
	factory.ItemRepository = repoMock

	someItem := provider.NewItem(iid)
	repoMock.On("Get", mock.AnythingOfType("*zilliqa.ZilliqaContract")).Return(someItem, nil).Once()
	repoMock.On("Update", mock.AnythingOfType("*zilliqa.ZilliqaContract"), []string{propertyName}).Return(nil).Once()

	//create job message
	itemId := &app.ItemId{
		ProvName:   providerName,
		ProvBranch: providerBranch,
		Id:         iid,
	}
	thejob := job.NewMessageJobPropertySet(providerKey, itemId, propertyName)

	payload, err := json.Marshal(thejob)
	assert.Nil(t, err)

	//restore job from marshaled message
	restoredJob, err := factory.UnmarshalJob(payload)
	assert.Nil(t, err)
	assert.Equal(t, "*job.JobPropertySet", reflect.TypeOf(restoredJob).String())

	newJobs, err := restoredJob.Execute()
	repoMock.AssertCalled(t, "Get", mock.AnythingOfType("*zilliqa.ZilliqaContract"))
	repoMock.AssertCalled(t, "Update", mock.AnythingOfType("*zilliqa.ZilliqaContract"), []string{propertyName})
	assert.Nil(t, err)
	assert.Nil(t, newJobs)
}
