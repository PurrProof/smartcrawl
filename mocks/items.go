package mocks

import (
	"purrproof/smartcrawl/app"

	"github.com/stretchr/testify/mock"
)

var _ app.IItemRepository = (*ItemRepositoryMock)(nil)

type ItemRepositoryMock struct {
	mock.Mock
}

func (m *ItemRepositoryMock) Get(item app.IItem) (app.IItem, error) {
	args := m.Called(item)
	return args.Get(0).(app.IItem), args.Error(1)
}

func (m *ItemRepositoryMock) GetAllWithoutProperty(provider app.IItemProvider, propName string, limit uint) ([]app.IItem, error) {
	return nil, nil
}

func (m *ItemRepositoryMock) Save(item app.IItem) error {
	return nil
}

func (m *ItemRepositoryMock) Update(item app.IItem, fieldNames []string) error {
	args := m.Called(item, fieldNames)
	return args.Error(0)
}

func (m *ItemRepositoryMock) Close() error {
	return nil
}
