package utils

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

//startLogger ...
func StartLogger(logPath string) {
	log.SetFlags(log.Lmicroseconds | log.Lshortfile)

	if _, err := os.Stat(logPath); err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(logPath, 0777)
		}
	}

	// fileName := fmt.Sprintf("/%s.log", filepath.Base(os.Args[0]))
	fileName := "/api.log"
	filePath := fmt.Sprintf(logPath+"logger/%d/%d/%d", time.Now().Year(), time.Now().Month(), time.Now().Day())
	WriteFile(fileName, filePath, []byte(``))

	logfile, err := os.OpenFile(filePath+fileName, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalln("Failed to open log file", ":", err)
	}
	log.SetFlags(log.Ldate | log.Lmicroseconds | log.Lshortfile)
	log.SetOutput(logfile)

	log.Println("::: logging started :::")
}

//WriteFile ...
func WriteFile(fileName, filePath string, fileBytes []byte) bool {
	// filePath = config.Get().Path + filePath
	_, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			os.MkdirAll(filePath, 0777)
		} else {
			return false
		}
	}

	if len(fileBytes) > 0 {
		file, err := os.Create(filePath + fileName)
		defer file.Close()
		if err != nil {
			log.Println("Failed Create Error", ":", err)
			return false
		}
		_, err = file.Write(fileBytes)

		if err != nil {
			log.Println("File Write Error: ", err)
			return false
		}
	}
	return true
}

//ParseFloat ...
func ParseFloat(str string) (float64, error) {
	val, err := strconv.ParseFloat(str, 64)
	if err == nil {
		return val, nil
	}

	//Some number may be seperated by comma, for example, 23,120,123, so remove the comma firstly
	str = strings.Replace(str, ",", "", -1)

	//Some number is specifed in scientific notation
	pos := strings.IndexAny(str, "eE")
	if pos < 0 {
		return strconv.ParseFloat(str, 64)
	}

	var baseVal float64
	var expVal int64

	baseStr := str[0:pos]
	baseVal, err = strconv.ParseFloat(baseStr, 64)
	if err != nil {
		return 0, err
	}

	expStr := str[(pos + 1):]
	expVal, err = strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		return 0, err
	}

	return baseVal * math.Pow10(int(expVal)), nil
}
