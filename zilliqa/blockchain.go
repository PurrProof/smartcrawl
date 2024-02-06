package zilliqa

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"purrproof/smartcrawl/app"
	"purrproof/smartcrawl/helpers"
	"strconv"
	"strings"
	"time"

	"github.com/Zilliqa/gozilliqa-sdk/bech32"
	contract2 "github.com/Zilliqa/gozilliqa-sdk/contract"
	"github.com/Zilliqa/gozilliqa-sdk/core"
	provider2 "github.com/Zilliqa/gozilliqa-sdk/provider"
	"github.com/Zilliqa/gozilliqa-sdk/transaction"
	transaction2 "github.com/Zilliqa/gozilliqa-sdk/transaction"
	"github.com/juju/errors"
	"github.com/sirupsen/logrus"

	"github.com/Zilliqa/gozilliqa-sdk/util"

	"github.com/Zilliqa/gozilliqa-sdk/account"
)

type ZilliqaApiConfig struct {
	HttpUrl              string
	TxConfrimMaxAttempts int
	TxConfirmIntervalSec int
}

type ZilliqaConfig struct {
	Id      string
	ChainId string
	Api     *ZilliqaApiConfig
}

const zeroAddress = "0000000000000000000000000000000000000000"

type ZilliqaBlockchain struct {
	Provider     *provider2.Provider
	Config       *ZilliqaConfig
	Wallet       *account.Wallet
	maxAttempts  int
	timeSleepSec time.Duration
}

var _ app.IItemProvider = (*ZilliqaBlockchain)(nil)

func NewZilliqaBlockchain(config *ZilliqaConfig) *ZilliqaBlockchain {
	prov := provider2.NewProvider(config.Api.HttpUrl)
	return &ZilliqaBlockchain{
		Provider:     prov,
		Config:       config,
		maxAttempts:  5,
		timeSleepSec: 2,
	}
}

func (z *ZilliqaBlockchain) PrepareItemsArray(limit uint) []app.IItem {
	result := make([]app.IItem, limit)
	for i := uint(0); i < limit; i++ {
		contract := &ZilliqaContract{}
		contract.Item = app.NewItem(z.Config.Id, z.Config.ChainId, "")
		result[i] = contract
	}
	return result
}

func (z *ZilliqaBlockchain) NewItem(id string) app.IItem {
	contract := &ZilliqaContract{}
	contract.Item = app.NewItem(z.Config.Id, z.Config.ChainId, id)
	contract.RegisterAutosetters()
	return contract
}

func (z *ZilliqaBlockchain) Close() error {
	return nil
}

func (z *ZilliqaBlockchain) GetContainersList(blocksNumber uint, startAfter *app.ItemsContainer) ([]*app.ItemsContainer, error) {
	var startBlock uint
	if blocksNumber == 0 {
		return []*app.ItemsContainer{}, nil
	} else if startAfter == nil {
		startBlock = 0
	} else {
		startBlock = startAfter.Uint() + 1
	}

	latestBlock, err := z.GetLatestBlockId()
	if err != nil {
		logrus.WithError(err).Error("can't get latest block id")
		return nil, errors.Annotate(err, "can't get latest block id")
	} else if startBlock > latestBlock {
		startBlock = latestBlock - 1
	}

	maxNumber := latestBlock - startBlock + 1
	if blocksNumber > maxNumber {
		blocksNumber = maxNumber
	}

	result := make([]*app.ItemsContainer, 0)
	for i := startBlock; i < startBlock+blocksNumber; i++ {
		blockId := strconv.Itoa(int(i))
		container := app.NewItemsContainer([]string{blockId})
		result = append(result, container)
	}

	logrus.WithFields(logrus.Fields{
		"start_block":        startBlock,
		"number_blocks":      blocksNumber,
		"latest_chain_block": latestBlock,
		"got_blocks":         len(result),
	}).Debug("GetContainersList")

	return result, nil
}

func (z *ZilliqaBlockchain) FetchContainerItems(container *app.ItemsContainer) ([]app.IItem, error) {

	idBlock := container.Uint()

	var contractsDeployed []app.IItem

	txArray, err := z.cycleGetTxnBodiesForTxBlock(idBlock)
	if err != nil {
		return nil, errors.Annotate(err, "can't fetch container items")
	}
	timestamp := uint32(0)

	for _, coreTx := range txArray {

		if !z.IsContractCreation(coreTx) {
			continue
		}
		if timestamp == 0 {
			//fetch timestamp once per block
			timestamp, err = z.getBlockTimestamp(idBlock)
			if err != nil {
				return nil, errors.Annotatef(err, "can't get timestamp for block=%d", idBlock)
			}
		}
		contractAddr, err := z.cycleGetContractAddressFromTransactionID(coreTx.ID)
		if err != nil {
			return nil, errors.Annotatef(err, "can't get contract address, txid=%s", coreTx.ID)
		}

		//init contract object
		contract := z.NewItem(contractAddr)
		contract.(*ZilliqaContract).Block = idBlock
		contract.(*ZilliqaContract).Txid = coreTx.ID
		contract.(*ZilliqaContract).Code = coreTx.Code
		contract.(*ZilliqaContract).Timestamp = timestamp

		contractsDeployed = append(contractsDeployed, contract)
	}

	return contractsDeployed, nil
}

func (z *ZilliqaBlockchain) getBlockTimestamp(idBlock uint) (uint32, error) {
	for attempt := 1; attempt < z.maxAttempts; attempt++ {
		fields := logrus.Fields{
			"api_call":      "GetTxBlock",
			"block_id":      idBlock,
			"network_error": false,
			"attempt":       attempt,
		}

		txBlock, err := z.Provider.GetTxBlock(strconv.Itoa(int(idBlock)))
		if err == nil {
			timestamp, err := strconv.Atoi(txBlock.Header.Timestamp[0:10])
			if err != nil {
				return uint32(0), errors.Annotatef(err, "can't get timestamp from string=", txBlock.Header.Timestamp[0:10])
			}
			return uint32(timestamp), nil
		} else if helpers.IsNetworkError(err) && attempt < z.maxAttempts {
			fields["network_error"] = true
			logrus.WithFields(fields).Warning(err)
			time.Sleep(z.timeSleepSec * time.Second)
			continue
		} else {
			//unexpected error
			logrus.WithFields(fields).WithError(err).Error("can't get block timestamp")
			return uint32(0), errors.Annotate(err, "can't get block timestamp")
		}
	}
	return uint32(0), errors.New("can't get block timestamp, max attempts reached")
}

func (z *ZilliqaBlockchain) cycleGetTxnBodiesForTxBlock(idBlock uint) ([]core.Transaction, error) {
	for attempt := 1; attempt < z.maxAttempts; attempt++ {
		fields := logrus.Fields{
			"api_call":      "GetTxnBodiesForTxBlock",
			"block_id":      idBlock,
			"network_error": false,
			"attempt":       attempt,
		}

		txArray, err := z.Provider.GetTxnBodiesForTxBlock(strconv.Itoa(int(idBlock)))
		if err == nil {
			return txArray, nil
		} else if strings.Contains(err.Error(), "TxBlock has no transactions") ||
			strings.Contains(err.Error(), "Txn Hash not Present") ||
			strings.Contains(err.Error(), "Failed to get Microblock") || //block 1664279
			strings.Contains(err.Error(), "Tx Block does not exist") {
			logrus.WithFields(logrus.Fields{
				"api_call": "GetTxnBodiesForTxBlock",
				"block_id": idBlock,
			}).Debug(err)
			return make([]core.Transaction, 0), nil
		} else if helpers.IsNetworkError(err) && attempt < z.maxAttempts {
			fields["network_error"] = true
			logrus.WithFields(fields).Warning(err)
			time.Sleep(2 * time.Second)
			continue
		} else {
			//unexpected error
			logrus.WithFields(fields).WithError(err).Error("can't get block transactions")
			return nil, errors.Annotate(err, "can't get block transactions")
		}
	}
	return nil, errors.New("can't get block transactions, max attempts reached")
}

func (z *ZilliqaBlockchain) cycleGetContractAddressFromTransactionID(txid string) (string, error) {
	for attempt := 1; attempt < z.maxAttempts; attempt++ {
		fields := logrus.Fields{
			"api_call":      "GetContractAddressFromTransactionID",
			"txid":          txid,
			"network_error": false,
			"attempt":       attempt,
		}

		contractAddr, err := z.Provider.GetContractAddressFromTransactionID(txid)

		if err == nil {
			return contractAddr, nil
		} else if helpers.IsNetworkError(err) && attempt < z.maxAttempts {
			fields["network_error"] = true
			logrus.WithFields(fields).Warning(err)
			time.Sleep(2 * time.Second)
			continue
		} else {
			//unexpected error
			logrus.WithFields(fields).WithError(err).Error("can't get contract address")
			return "", errors.Annotate(err, "can't get contract address")
		}
	}
	return "", errors.New("can't get contract address, max attempts reached")
}

func (z *ZilliqaBlockchain) IsContractCreation(txn core.Transaction) bool {
	if txn.ToAddr == zeroAddress && txn.Receipt.Success == true && txn.Code != "" {
		return true
	}
	return false
}

func (z *ZilliqaBlockchain) GetLatestBlockId() (uint, error) {
	result, err := z.Provider.GetNumTxBlocks()
	if err != nil {
		return 0, errors.Annotate(err, "can't get blockhain height")
	}
	res, _ := strconv.Atoi(result)
	return uint(res), nil
}

/*func (z *ZilliqaBlockchain) RestoreContract(contractAddress string) (*app.IItem, error) {
	//TODO: errors.Trace/Annotate
	////b32, err := bech32.ToBech32Address(contractAddress)
	////if err != nil {
	////	return nil, err
	////}

	//check whether contract is deployed
	addr := contractAddress
	if contractAddress[0:2] == "0x" {
		addr = addr[2:]
	}

	////result,err
	_, err := z.Provider.GetSmartContractInit(addr)
	if err != nil {
		return nil, err
	}

	contract := ZilliqaContract{...}
	//we can restore also contract code, state, creation tx id and other parameters

	return contract, nil
}*/

func (z *ZilliqaBlockchain) SetWallet(newKey string) error {
	//TODO: errors.Trace/Annotate
	if newKey == "" {
		//todo: better validation
		return fmt.Errorf("Wrong private key")
	}
	wallet := account.NewWallet()
	wallet.AddByPrivateKey(newKey)
	z.Wallet = wallet
	return nil
}

func (z *ZilliqaBlockchain) State(contractAddress string) (string, error) {
	//TODO: errors.Trace/Annotate
	rsp, err := z.Provider.GetSmartContractState(contractAddress[2:])
	if err != nil {
		return "", err
	}
	result, err := json.MarshalIndent(rsp.Result, "", "     ")
	if err != nil {
		return "", err
	}
	state := string(result)
	return state, nil
}

func (z *ZilliqaBlockchain) SubState(contractAddress string, params ...interface{}) (string, error) {
	//TODO: errors.Trace/Annotate
	rsp, err := z.Provider.GetSmartContractSubState(contractAddress, params...)
	if err != nil {
		return "", err
	}

	state := string(rsp)

	return state, nil
}

func (z *ZilliqaBlockchain) BuildBatchParams(fields []string) [][]interface{} {
	//TODO: errors.Trace/Annotate
	var params [][]interface{}

	for _, v := range fields {
		params = append(params, []interface{}{v, []string{}})
	}

	return params
}

// Inspired by https://github.com/Zilliqa/gozilliqa-sdk/blob/2ff222c97fc6fa2855ef2c5bffbd56faddd6291f/provider/provider.go#L877
func (z *ZilliqaBlockchain) BatchSubState(contractAddress string, fields []string) (string, error) {
	//TODO: errors.Trace/Annotate

	params := z.BuildBatchParams(fields)

	//we should hack here for now
	type req struct {
		Id      string      `json:"id"`
		Jsonrpc string      `json:"jsonrpc"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}

	reqs := []*req{}

	for i, param := range params {
		p := []interface{}{
			contractAddress[2:],
		}

		for _, v := range param {
			p = append(p, v)
		}

		r := &req{
			Id:      strconv.Itoa(i + 1),
			Jsonrpc: "2.0",
			Method:  "GetSmartContractSubState",
			Params:  p,
		}

		reqs = append(reqs, r)
	}

	b, _ := json.Marshal(reqs)
	reader := bytes.NewReader(b)
	request, err := http.NewRequest("POST", z.Config.Api.HttpUrl, reader)
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json;charset=UTF-8")
	client := http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	result, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	//response needs to be parsed, because error is possible
	//[{"error":{"code":-8,"data":null,"message":"Address size not appropriate"},"id":"1","jsonrpc":"2.0"},{"error":{"code":-8,"data":null,"message":"Address size not appropriate"},"id":"2","jsonrpc":"2.0"}]

	return string(result), nil
}

func (z *ZilliqaBlockchain) Deploy(code string, init []core.ContractValue) (*transaction2.Transaction, error) {
	//TODO: errors.Trace/Annotate

	if z.Wallet == nil {
		return nil, errors.New("wallet not set")
	}

	gasPrice, err := z.Provider.GetMinimumGasPrice()
	if err != nil {
		return nil, err
	}

	chainId, err := strconv.Atoi(z.Config.ChainId)
	if err != nil {
		return nil, err
	}
	parameter := contract2.DeployParams{
		Version:      strconv.FormatInt(int64(util.Pack(chainId, 1)), 10),
		Nonce:        "",
		GasPrice:     gasPrice,
		GasLimit:     "75000",
		SenderPubKey: "",
	}

	contract := contract2.Contract{
		Provider: z.Provider,
		Code:     code,
		Init:     init,
		Signer:   z.Wallet,
	}

	tx, err := contract.Deploy(parameter)
	if err != nil {
		return tx, err
	}

	tx.Confirm(tx.ID, z.Config.Api.TxConfrimMaxAttempts, z.Config.Api.TxConfirmIntervalSec, z.Provider)
	if tx.Status != core.Confirmed {
		data, _ := json.MarshalIndent(tx.Receipt, "", "     ")
		return tx, fmt.Errorf("deploy failed: %s", string(data))
	}

	return tx, nil
}

func (z *ZilliqaBlockchain) Call(address, transition string, args []core.ContractValue, amount string) (*transaction.Transaction, error) {
	//TODO: errors.Trace/Annotate

	b32, err := bech32.ToBech32Address(address)
	if err != nil {
		return nil, err
	}

	contract := contract2.Contract{
		Provider: z.Provider,
		Address:  b32,
		Signer:   z.Wallet,
	}

	gasPrice, err := z.Provider.GetMinimumGasPrice()
	if err != nil {
		return nil, err
	}
	priority := false
	chainId, err := strconv.Atoi(z.Config.ChainId)
	if err != nil {
		return nil, err
	}
	params := contract2.CallParams{
		Version:      strconv.FormatInt(int64(util.Pack(chainId, 1)), 10),
		Nonce:        "",
		GasPrice:     gasPrice,
		GasLimit:     "40000",
		Amount:       amount,
		SenderPubKey: "",
	}
	tx, err := contract.Call(transition, args, params, priority)
	if err != nil {
		return tx, err
	}

	tx.Confirm(tx.ID, z.Config.Api.TxConfrimMaxAttempts, z.Config.Api.TxConfirmIntervalSec, z.Provider)
	if tx.Status != core.Confirmed {
		err := errors.New("transaction didn't get confirmed")
		if len(tx.Receipt.Exceptions) > 0 {
			err = errors.New(tx.Receipt.Exceptions[0].Message)
		}
		return tx, err
	}
	if !tx.Receipt.Success {
		return tx, errors.New("transaction failed")
	}
	return tx, nil
}
