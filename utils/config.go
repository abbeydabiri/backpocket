package utils

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/jmoiron/sqlx"
	//needed for postgresql
	_ "github.com/lib/pq"

	"github.com/spf13/viper"
	"golang.org/x/crypto/nacl/secretbox"
)

const (
	keySize   = 32
	nonceSize = 24
)

//Config structure
type configType struct {
	Timezone, Cookie, Path,
	Address, OS, APIkey,
	APISecret string

	Encryption struct {
		Private []byte
		Public  []byte
	}

	Crex24  struct{ Key, Secret string }
	Binance struct{ Key, Secret string }

	dbConfig map[string]string

	CGate, CSplash map[string]string
}

//Config to be exported globally
var (
	Config configType
	SqlDB  *sqlx.DB
)

//Init ...
func Init(yamlConfig []byte) {

	viper.SetConfigType("yaml")
	viper.SetDefault("address", "127.0.0.1:8080")

	var err error
	if yamlConfig == nil {
		viper.SetConfigName("config")
		viper.AddConfigPath("./")  // optionally look for config in the working directory
		err = viper.ReadInConfig() // Find and read the config file
	} else {
		err = viper.ReadConfig(bytes.NewBuffer(yamlConfig))
	}

	if err != nil { // Handle errors reading the config file
		panic(fmt.Errorf("fatal error config file: %s", err))
	}

	Config = configType{}
	Config.OS = viper.GetString("os")
	Config.Path = viper.GetString("path")
	Config.Cookie = viper.GetString("cookie")
	Config.Address = viper.GetString("address")
	Config.Timezone = viper.GetString("timezone")

	if crex24Map := viper.GetStringMapString("crex24"); crex24Map != nil {
		Config.Crex24.Key = crex24Map["key"]
		Config.Crex24.Secret = crex24Map["secret"]
	}

	if binanceMap := viper.GetStringMapString("binance"); binanceMap != nil {
		Config.Binance.Key = binanceMap["key"]
		Config.Binance.Secret = binanceMap["secret"]
	}

	encrptionKeysMap := viper.GetStringMapString("encryption_keys")
	if encrptionKeysMap != nil {
		Config.Encryption.Public, err = Asset(encrptionKeysMap["public"])
		if err != nil {
			log.Fatalf("Error reading public key %v", err)
			return
		}

		Config.Encryption.Private, err = Asset(encrptionKeysMap["private"])
		if err != nil {
			log.Fatalf("Error reading private key %v", err)
			return
		}
	}

	//SQL Connection for POSTGRESQL
	if Config.dbConfig = viper.GetStringMapString("dbconfig"); len(Config.dbConfig) >= 5 {
		ConnectDB()
	}
	//SQL Connection for POSTGRESQL
}

func ConnectDB() {

	var err error
	//SQL Connection for POSTGRES
	postgresConn := "host=%s port=%s dbname=%s user=%s password=%s sslmode=%s connect_timeout=5"
	postgresConn = fmt.Sprintf(postgresConn, Config.dbConfig["hostname"], Config.dbConfig["port"],
		Config.dbConfig["database"], Config.dbConfig["username"], Config.dbConfig["password"], Config.dbConfig["sslmode"])

	if SqlDB, err = sqlx.Open("postgres", postgresConn); err != nil {
		log.Panicf("error opening database file %v \n", err)
	}
	//SQL Connection for POSTGRES
}

//Encrypt ...
func Encrypt(in []byte) (out []byte) {
	key, nonce := keyNounce()
	out = secretbox.Seal(out, in, nonce, key)
	return
}

//Decrypt ...
func Decrypt(in []byte) (out []byte) {
	key, nonce := keyNounce()
	out, _ = secretbox.Open(out, in, nonce, key)
	return
}

func spaceRemove(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}

func keyNounce() (key *[keySize]byte, nonce *[nonceSize]byte) {
	fullPath := filepath.Dir(os.Args[0])
	fullPath = spaceRemove(fullPath)
	fullPath = strings.Replace(fullPath, "/", "", -1)
	fullPath = strings.Replace(fullPath, "\\", "", -1)

	fullPath = base64.StdEncoding.EncodeToString([]byte(fullPath))
	nPower := int(60 / len(fullPath))
	if len(fullPath) < 60 {
		nCount := 0
		for nPower > nCount {
			fullPath += fullPath
			nCount++
		}
		fullPath = fullPath[0:60]
	}

	key = new([keySize]byte)
	copy(key[:], []byte(fullPath[0:32])[:keySize])

	nonce = new([nonceSize]byte)
	copy(nonce[:], []byte(fullPath[0:32][0:24])[:nonceSize])

	return
}
