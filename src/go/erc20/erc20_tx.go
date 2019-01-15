package erc20

import  (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"

	"github.com/fusion/go-fusion/params"


	"github.com/fusion/go-fusion"
	"github.com/fusion/go-fusion/accounts/abi"
	"github.com/fusion/go-fusion/accounts/abi/bind"
	"github.com/fusion/go-fusion/common"
	"github.com/fusion/go-fusion/core/types"
	ethcrypto "github.com/fusion/go-fusion/crypto"
	"github.com/fusion/go-fusion/crypto/sha3"
	"github.com/fusion/go-fusion/ethclient"

	"github.com/cryptocoins/src/go/config"
	rpcutils "github.com/cryptocoins/src/go/rpcutils"

	"github.com/cryptocoins/src/go/erc20/token"
)

var (
	url = config.ETH_GATEWAY
	err error
	chainConfig = params.RinkebyChainConfig
)

var tokens map[string]string = map[string]string{
	"GUSD":"0x28a79f9b0fe54a39a0ff4c10feeefa832eeceb78",
	"BNB":"0x7f30B414A814a6326d38535CA8eb7b9A62Bceae2",
	"MKR":"0x2c111ede2538400F39368f3A3F22A9ac90A496c7",
	"HT":"0x3C3d51f6BE72B265fe5a5C6326648C4E204c8B9a",
	"BNT":"0x14D5913C8396d43aB979D4B29F2102c1C65E18Db",
}

type ERC20TransactionHandler struct {
}

func (h ERC20TransactionHandler) PublicKeyToAddress (pubKeyHex string) (address string, msg string, err error) {
	data := hexEncPubkey(pubKeyHex[2:])

	pub, err := decodePubkey(data)

	address = ethcrypto.PubkeyToAddress(*pub).Hex()
	return
}

//args[0]: gasPrice	*big.Int
//args[1]: gasLimit	uint64
//args[2]: tokenType	string
func (h ERC20TransactionHandler) BuildUnsignedTransaction (fromAddress, fromPublicKey, toAddress string, amount *big.Int, args ...interface{}) (transaction interface{}, digests []string, err error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return
	}
	transaction, hash, err := erc20_newUnsignedTransaction(client, fromAddress, toAddress, amount, args[0].(*big.Int), args[1].(uint64), args[2].(string))
	hashStr := hash.Hex()
	if hashStr[:2] == "0x" {
		hashStr = hashStr[2:]
	}
	digests = append(digests, hashStr)
	return
}

/*
func SignTransaction(hash string, address string) (rsv string, err error) {
	return
}
*/

func (h ERC20TransactionHandler) SignTransaction(hash []string, privateKey interface{}) (rsv []string, err error) {
	hashBytes, err := hex.DecodeString(hash[0])
	if err != nil {
		return
	}
	r, s, err := ecdsa.Sign(rand.Reader, privateKey.(*ecdsa.PrivateKey), hashBytes)
	if err != nil {
		return
	}
	fmt.Printf("r: %v\ns: %v\n\n", r, s)
	rx := fmt.Sprintf("%X", r)
	sx := fmt.Sprintf("%X", s)
	rsv = append(rsv, rx + sx + "00")
	return
}

func (h ERC20TransactionHandler) MakeSignedTransaction(rsv []string, transaction interface{}) (signedTransaction interface{}, err error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return
	}
	return makeSignedTransaction(client, transaction.(*types.Transaction), rsv[0])
}

func (h ERC20TransactionHandler) SubmitTransaction(signedTransaction interface{}) (ret string, err error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return
	}
	return erc20_sendTx(client, signedTransaction.(*types.Transaction))
}

func (h ERC20TransactionHandler) GetTransactionInfo(txhash string) (fromAddress, toAddress string, transferAmount *big.Int, _ []interface{}, err error) {
	client, err := ethclient.Dial(url)
	if err != nil {
		return
	}
	hash := common.HexToHash(txhash)
	tx, isPending, err1 := client.TransactionByHash(context.Background(), hash)
	if err1 == nil && isPending == false && tx != nil {
		msg, err2 := tx.AsMessage(types.MakeSigner(chainConfig, big.NewInt(3504188)))
		err = err2
		fromAddress = msg.From().Hex()
		data := msg.Data()

		toAddress, transferAmount, err = DecodeTransferData(data)

	} else if err1 != nil {
		err = err1
	} else if isPending {
		err = fmt.Errorf("Transaction is pending")
	} else {
		err = fmt.Errorf("Unknown error")
	}
	return
}

// args[0] coinType string
func (h ERC20TransactionHandler) GetAddressBalance(address string, args ...interface{}) (balance *big.Int, err error) {
	if args[0] == nil {
		err = fmt.Errorf("Unspecified coin type")
		return
	}

	tokenAddr := tokens[args[0].(string)]
	if tokenAddr == "" {
		err = fmt.Errorf("Token not supported")
		return
	}

	myABIJson := `[{"constant":true,"inputs":[{"name":"_owner","type":"address"}],"name":"balanceOf","outputs":[{"name":"balance","type":"uint256"}],"payable":false,"stateMutability":"view","type":"function"},{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"payable":false,"stateMutability":"view","type":"function"}]`
	myABI, err := abi.JSON(strings.NewReader(myABIJson))
	if err != nil {
		return
	}

	data, err := myABI.Pack("balanceOf", common.HexToAddress(address))
	if err != nil {
		return
	}
	dataHex := "0x" + hex.EncodeToString(data)
	fmt.Printf("data is %v\n\n", dataHex)

	reqJson := `{"jsonrpc": "2.0","method": "eth_call","params": [{"to": "` + tokenAddr + `","data": "` + dataHex + `"},"latest"],"id": 1}`
	fmt.Printf("reqJson: %v\n\n", reqJson)

	ret := rpcutils.DoPostRequest2(url, reqJson)
	fmt.Printf("ret: %v\n\n", ret)

	var retStruct map[string]interface{}
	json.Unmarshal([]byte(ret), &retStruct)
	if retStruct["result"] == nil {
		if retStruct["error"] != nil {
			err = fmt.Errorf(retStruct["error"].(map[string]interface{})["message"].(string))
			return
		}
		err = fmt.Errorf(ret)
		return
	}
	balanceStr := retStruct["result"].(string)[2:]
	balanceHex, _ := new(big.Int).SetString(balanceStr, 16)
	balance, _ = new(big.Int).SetString(fmt.Sprintf("%d",balanceHex), 10)

	client, err := ethclient.Dial(url)
	if err != nil {
		log.Fatal(err)
	}

	tokenAddress := common.HexToAddress(tokenAddr)
	instance, err := token.NewToken(tokenAddress, client)
	if err != nil {
		log.Fatal(err)
	}

	balance1, _ := instance.BalanceOf(&bind.CallOpts{}, common.HexToAddress(address))
	fmt.Printf("balance1: %v\n\n", balance1)

	return
}

func hexEncPubkey(h string) (ret [64]byte) {
	b, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	if len(b) != len(ret) {
		panic("invalid length")
	}
	copy(ret[:], b)
	return ret
}


func decodePubkey(e [64]byte) (*ecdsa.PublicKey, error) {
	p := &ecdsa.PublicKey{Curve: ethcrypto.S256(), X: new(big.Int), Y: new(big.Int)}
	half := len(e) / 2
	p.X.SetBytes(e[:half])
	p.Y.SetBytes(e[half:])
	if !p.Curve.IsOnCurve(p.X, p.Y) {
		return nil, errors.New("invalid secp256k1 curve point")
	}
	return p, nil
}

func DecodeTransferData(data []byte) (toAddress string, transferAmount *big.Int, err error) {
	eventData := data[:4]
	if string(eventData) == string([]byte{0xa9, 0x05, 0x9c, 0xbb}) {
		addressData := data[4:36]
		amountData := data[36:]
		num, _ := new(big.Int).SetString(hex.EncodeToString(addressData), 16)
		toAddress = "0x" + fmt.Sprintf("%x", num)
		amountHex, _ := new(big.Int).SetString(hex.EncodeToString(amountData), 16)
		transferAmount, _ = new(big.Int).SetString(fmt.Sprintf("%d", amountHex), 10)
	} else {
		err = fmt.Errorf("Invalid transfer data")
		return
	}
	return
}

func erc20_newUnsignedTransaction (client *ethclient.Client, dcrmAddress string, toAddressHex string, amount *big.Int, gasPrice *big.Int, gasLimit uint64, tokenType string) (*types.Transaction, *common.Hash, error) {

	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, nil, err
	}

	tokenAddressHex, ok := tokens[tokenType]
	if ok {
	} else {
		err = errors.New("token not supported")
		return nil, nil, err
	}

	if gasPrice == nil {
		gasPrice, err = client.SuggestGasPrice(context.Background())
		if err != nil {
			return nil, nil, err
		}
	}

	fromAddress := common.HexToAddress(dcrmAddress)
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return nil, nil, err
	}

	value := big.NewInt(0)

	toAddress := common.HexToAddress(toAddressHex)
	tokenAddress := common.HexToAddress(tokenAddressHex)

	transferFnSignature := []byte("transfer(address,uint256)")
	hash := sha3.NewKeccak256()
	hash.Write(transferFnSignature)
	methodID := hash.Sum(nil)[:4]

	paddedAmount := common.LeftPadBytes(amount.Bytes(), 32)
	paddedAddress := common.LeftPadBytes(toAddress.Bytes(), 32)

	var data []byte
	data = append(data, methodID...)
	data = append(data, paddedAddress...)
	data = append(data, paddedAmount...)

	if gasLimit <= 0 {
		gasLimit, err = client.EstimateGas(context.Background(), ethereum.CallMsg{
			To:   &tokenAddress,
			Data: data,
		})
		gasLimit = gasLimit * 4
		if err != nil {
			return nil, nil, err
		}
	}

	fmt.Println("gasLimit is ", gasLimit)
	fmt.Println("gasPrice is ", gasPrice)
	tx := types.NewTransaction(nonce, tokenAddress, value, gasLimit, gasPrice, data)

	signer := types.NewEIP155Signer(chainID)
	txhash := signer.Hash(tx)
	return tx, &txhash, nil
}

func makeSignedTransaction(client *ethclient.Client, tx *types.Transaction, rsv string) (*types.Transaction, error) {
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return nil, err
	}
	message, err := hex.DecodeString(rsv)
	if err != nil {
		return nil, err
	}
	signer := types.NewEIP155Signer(chainID)
	signedtx, err := tx.WithSignature(signer, message)
	if err != nil {
		return nil, err
	}
	return signedtx, nil
}

func erc20_sendTx (client *ethclient.Client, signedTx *types.Transaction) (string, error) {
	err := client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}
	return "success/" + signedTx.Hash().Hex(), nil
}