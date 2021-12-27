package utils

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"golang.org/x/mobile/asset"
)

//Asset ...
func Asset(filename string) (assetByte []byte, assetError error) {
	if strings.HasSuffix(filename, "/") {
		assetError = fmt.Errorf("directory listing forbidden")
	} else {

		switch Config.OS {
		case "ios", "android":
			switch {
			case
				filename == "public.pem",
				filename == "private.pem",
				strings.HasPrefix(filename, "frontend/"):
				if f, errOpen := asset.Open(filename); errOpen == nil {
					defer f.Close()
					assetByte, assetError = ioutil.ReadAll(f)
				}

			default:
				assetByte, assetError = ioutil.ReadFile(Config.Path + filename)
			}
		default:
			assetByte, assetError = ioutil.ReadFile(Config.Path + filename)
		}
	}
	return
}

//AssetDir ...
func AssetDir(fileDir string) (assetString []string, assetError error) {
	var filePath string
	switch Config.OS {
	case "ios", "android":
		filePath = Config.Path + fileDir
	default:
		filePath = "." + fileDir
	}
	fileInfos, err := ioutil.ReadDir(filePath)
	assetString = make([]string, len(fileInfos))
	for counter, file := range fileInfos {
		assetString[counter] = file.Name()
	}
	assetError = err
	return
}

//AssetRemove ...
func AssetRemove(filePath string) (assetError error) {
	filePath = Config.Path + filePath
	if err := os.Remove(filePath); err != nil {
		log.Printf("AssetRemove: %v", err.Error())
	}
	return
}

//AssetDirList ...
func AssetDirList(fileDir string) (assetString []string, assetError error) {
	fileDir = Config.Path + fileDir
	fileInfos, err := ioutil.ReadDir(fileDir)
	assetString = make([]string, len(fileInfos))
	for counter, file := range fileInfos {
		assetString[counter] = file.Name()
	}
	assetError = err
	return
}

//AssetDirRemove ...
func AssetDirRemove(fileDir string) (assetError error) {
	fileDir = Config.Path + fileDir
	_, assetError = os.Stat(fileDir)
	if assetError == nil {
		os.RemoveAll(fileDir)
		if err := os.Remove(fileDir); err != nil {
			log.Printf("AssetDirRemove: %v", err.Error())
		}
	}
	return
}
