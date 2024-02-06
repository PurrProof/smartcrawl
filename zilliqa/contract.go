package zilliqa

import (
	"purrproof/smartcrawl/app"
	"regexp"
)

type ZilliqaContract struct {
	*app.Item `bson:"inline"`
	//from app.Item:
	//ProvName              string //Zilliqa
	//ProvBranch           string //ChainId
	//Id                  string //Address
	Block     uint   `bson:"block"`
	Txid      string `bson:"txid"`
	Code      string `bson:"code"`
	Timestamp uint32 `bson:"timestamp"`
	//realtime computed properties
	Name      string `bson:"name"`
	Library   string `bson:"library"`
	SizeBytes int    `bson:"sizebytes"`
	//delayed computed properties
	//Test  string `bson:"test"`
}

var _ app.IItem = (*ZilliqaContract)(nil)

func (c *ZilliqaContract) RegisterAutosetters() error {
	c.RegisterRealtimeAutosetter("SizeBytes", c.AutosetSizeBytes)
	c.RegisterRealtimeAutosetter("Name", c.AutosetName)
	c.RegisterRealtimeAutosetter("Library", c.AutosetLibrary)
	//c.RegisterDelayedAutosetter("Test", c.AutosetTest)
	return nil
}

/* ========== realtime computed properties ========== */

func (c *ZilliqaContract) AutosetSizeBytes() error {
	size := len(c.Code)
	c.SizeBytes = size
	return nil
}

//contract name
func (c *ZilliqaContract) AutosetName() error {
	re := regexp.MustCompilePOSIX(`contract[ \n\r\t]*([A-Za-z0-9_]+)[ \n\r\t]*\(`)
	result := re.FindStringSubmatch(c.Code)
	if len(result) > 0 {
		c.Name = result[1]
	} else {
		c.Name = ""
	}
	return nil
}

func (c *ZilliqaContract) AutosetLibrary() error {
	re := regexp.MustCompilePOSIX(`library[ \n\r\t]*([A-Za-z0-9_]+)[ \n\r\t]+`)
	result := re.FindStringSubmatch(c.Code)
	if len(result) > 0 {
		c.Library = result[1]
	} else {
		//sometimes there is no library really, e.g. https://viewblock.io/zilliqa/address/zil1f7lwjv7suu0e908mzqdxcthpl6mn08qfgtn7tu?tab=state
		c.Library = ""
	}
	return nil
}

/* ========== delayed computed properties ========== */
/*func (c *ZilliqaContract) AutosetTest() error {
	c.Test = "foo"
	return nil
}*/
