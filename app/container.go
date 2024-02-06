package app

import (
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

type ItemsContainer struct {
	Id []string
	// Previously, it was like this, but JSON unmarshals numbers into float64 by default, it's easier for me to work with strings for now.
	// Id []interface{}
	/*
	For Zilliqa, the ID is the block number, one element in the array.
	Possibly for other types of containers, there will be a composite ID.
	*/
}

func NewItemsContainer(id []string) *ItemsContainer {
	return &ItemsContainer{
		Id: id,
	}
}

func (c *ItemsContainer) GetId() []string {
	return c.Id
}

func (c *ItemsContainer) String() string {
	return strings.Join(c.Id, "_")
}

func (c *ItemsContainer) Uint() uint {
	id := c.GetId()
	if len(id) != 1 {
		logrus.WithFields(logrus.Fields{
			"id_stringified": c.String(),
		}).Warning("can't convert composite Id to uint correctly, return 0")
		return 0
	}
	idInt, err := strconv.Atoi(id[0])
	if err != nil {
		logrus.WithFields(logrus.Fields{
			"id_stringified": c.String(),
		}).Warning("can't convert not numeric Id uint correctly, return 0")
		return 0
	}
	return uint(idInt)
}

func (c *ItemsContainer) SetId(id []string) {
	c.Id = id
}
