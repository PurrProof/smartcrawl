package factory

import (
	"purrproof/smartcrawl/app"
	"purrproof/smartcrawl/zilliqa"
	"strings"

	"github.com/juju/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/sirupsen/logrus"
)

func (f *Factory) GetProviderByKey(provKey string) (app.IItemProvider, error) {
	// Keys in the config map are in lowercase, as Viper reads them
	provKeyLower := strings.ToLower(provKey)
	if prov, found := f.ItemProvider[provKeyLower]; found {
		return prov, nil
	}

	pconf, err := f.AppConfig.GetProviderConfigByKey(provKey)
	if err != nil {
		return nil, errors.Annotatef(err, "can't get provider config for id=%s", provKey)
	}
	var itemProv app.IItemProvider
	switch provKeyLower {
	case "zilmain":
		zconfig := zilliqa.ZilliqaConfig{}
		mapstructure.Decode(pconf, &zconfig)
		itemProv = zilliqa.NewZilliqaBlockchain(&zconfig)
	default:
		return nil, errors.Errorf("unknown provider: %s", provKey)
	}

	logrus.WithFields(logrus.Fields{
		"provider": provKey,
	}).Debug("provider initialized")

	f.ItemProvider[provKeyLower] = itemProv
	f.Defer(f.ItemProvider[provKeyLower].Close)
	return itemProv, nil
}
