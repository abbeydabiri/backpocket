package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"unsafe"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	// "github.com/ethereum/go-ethereum/crypto/sha3"
)

var err error

//Client ...
var Client *ethclient.Client

//WEI ...
const WEI = 1000000000000000000

//ETHAddressNum ...
var ETHAddressNum = 0

//ETHGasLimit ...
var ETHGasLimit = uint64(6009850)

//InfuraKey ...
//var InfuraNetwork = "https://rinkeby.infura.io/wvxLGQSZBjP3Ak7iqt8J"
var InfuraNetwork = "https://rinkeby.infura.io/v3/349d1750fa35425d9625f0fa1e03895e"

const infuraKey = "wvxLGQSZBjP3Ak7iqt8J"
const infuraKovan = "https://kovan.infura.io"
const infuraRopsten = "https://ropsten.infura.io"
const infuraRinkeby = "https://rinkeby.infura.io"
const infuraMainnet = "https://mainnet.infura.io"

func EthHexToFloat64(hex string) (float64, error) {
	hexUint, err := strconv.ParseUint(hex, 16, 64)
	if err != nil {
		log.Println(err.Error())
		return 0.0, err
	}

	hexFloat := uint64(hexUint)
	hexResult := *(*float64)(unsafe.Pointer(&hexFloat))
	return hexResult, nil
}

//EthClientDial ...
func EthClientDial(network string) {
	network = InfuraNetwork
	Client, err = ethclient.Dial(network)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("we have a connection to Infura network: " + network)
	_ = Client
}

//ETHNewMnemonic ...
func ETHNewMnemonic() string {
	// Generate a mnemonic for memorization or user-friendly seeds
	entropy, _ := bip39.NewEntropy(512)
	mnemonic, _ := bip39.NewMnemonic(entropy)
	return mnemonic
}

//ETHIsMnemonicValid ...
func ETHIsMnemonicValid(mnemonic string) bool {
	return bip39.IsMnemonicValid(mnemonic)
}

//EthGenerateKey ...
func EthGenerateKey(mnemonic string, level int) (privateKey *ecdsa.PrivateKey, fromAddress common.Address) {

	seed := bip39.NewSeed(mnemonic, "")
	masterPrivateKey, _ := bip32.NewMasterKey(seed)

	// masterPublicKey := masterPrivateKey.PublicKey()
	// Display mnemonic and keys
	// fmt.Printf("Mnemonic: [%v] \n",mnemonic)
	// fmt.Println("Master private key: ", masterPrivateKey)
	// fmt.Println("Master public key: ", masterPublicKey.PublicKey())

	const Purpose uint32 = 0x8000002C
	const CoinEther uint32 = 0x8000003c
	const Account uint32 = 0x80000000
	const SubAccounts uint32 = 0x00000000 //this must be uint32 0

	child, err := masterPrivateKey.NewChildKey(Purpose)
	if err != nil {
		log.Println("Purpose error: ", err.Error())
		return
	}

	child, err = child.NewChildKey(CoinEther)
	if err != nil {
		log.Println("CoinEther error: ", err.Error())
		return
	}

	child, err = child.NewChildKey(Account)
	if err != nil {
		log.Println("Account error: ", err.Error())
		return
	}

	childAccounts, _ := child.NewChildKey(SubAccounts)
	if err != nil {
		log.Println("External error: ", err.Error())
		return
	}

	childPrivateKey, _ := childAccounts.NewChildKey(uint32(level))

	privateKeyHex := hexutil.Encode(childPrivateKey.Key)[2:]
	privateKey, _ = crypto.HexToECDSA(privateKeyHex)
	fromAddress = crypto.PubkeyToAddress(privateKey.PublicKey)

	return
}

//EthAccountTransfer ...
func EthAccountTransfer(amount float64, fromAddress, toAddress common.Address, privateKey *ecdsa.PrivateKey) {
	//Get the Nonce
	nonce, err := Client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		log.Fatal(err)
	}

	value := big.NewInt(int64(amount * WEI)) // in WEI (1 eth)
	gasLimit := uint64(21000)                // in units
	gasPrice, err := Client.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var data []byte
	tx := types.NewTransaction(nonce, toAddress, value, gasLimit, gasPrice, data)
	signedTx, err := types.SignTx(tx, types.HomesteadSigner{}, privateKey)
	if err != nil {
		log.Fatal(err)
	}

	err = Client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("tx sent: https://rinkeby.etherscan.io/tx/%s \n", signedTx.Hash().Hex())
}

//EthAccountBal ...
func EthAccountBal(network, address string, block int64) (balance *big.Int, err error) {
	if Client == nil {
		EthClientDial(network)
	}

	if Client != nil {
		var blockNumber *big.Int
		if block > 0 {
			blockNumber = big.NewInt(block)
		}
		account := common.HexToAddress(address)
		balance, err = Client.BalanceAt(context.Background(), account, blockNumber)

		if err != nil {
			log.Println("Account Balannce error: ", err.Error())
		}
	}
	return
}

//ETHAccountBalFloat ...
func ETHAccountBalFloat(network, address string, block int64) (*big.Float, error) {
	bal, err := EthAccountBal(network, address, block)
	balFloat, weiFloat := new(big.Float).SetInt(bal), big.NewFloat(WEI)
	newBalance := new(big.Float).Quo(balFloat, weiFloat)
	return newBalance, err
}
