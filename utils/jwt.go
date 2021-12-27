package utils

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
)

//VerifyJWT ...
func VerifyJWT(httpRes http.ResponseWriter, httpReq *http.Request) (claims jwt.MapClaims) {

	if monsterCookie, err := httpReq.Cookie(Config.Cookie); err == nil {
		claims = ValidateJWT(monsterCookie.Value)
		if claims == nil {
			cookieMonster := &http.Cookie{
				Name: Config.Cookie, Value: "deleted", Path: "/",
				Expires: time.Now().Add(-(time.Hour * 24 * 30 * 12)), // set the expire time
				// SameSite: http.SameSiteNoneMode, Secure: true,
			}
			http.SetCookie(httpRes, cookieMonster)
			//httpReq.AddCookie(cookieMonster)
		}
	} else {
		log.Println(err.Error())
	}

	return
}

//ValidateJWT ...
func ValidateJWT(jwtToken string) (claims jwt.MapClaims) {
	token, _ := jwt.Parse(jwtToken, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		publicKey, _ := jwt.ParseRSAPublicKeyFromPEM(Config.Encryption.Public)
		return publicKey, nil
	})

	if token != nil {
		jwtClaims, ok := token.Claims.(jwt.MapClaims)
		if ok && token.Valid {
			if jwtClaims["claims"] != nil {
				base64Bytes, err := base64.StdEncoding.DecodeString(
					jwtClaims["claims"].(string))
				if err != nil {
					log.Println("error: " + err.Error())
					return
				}
				if base64Bytes == nil {
					log.Println("base64Bytes is nil")
					return
				}
				byteClaims := Decrypt(base64Bytes)
				json.Unmarshal(byteClaims, &claims)
				if claims["exp"] != nil {
					claims["exp"] = int64(claims["exp"].(float64))
				}
			}
		}
	}
	return
}

//GenerateJWT Turn user details into a hashed token that can be used to recognize the user in the future.
func GenerateJWT(claims jwt.MapClaims) (token string, err error) {

	//create new claims with encrypted data
	jwtClaims := jwt.MapClaims{}
	byteClaims, _ := json.Marshal(claims)
	jwtClaims["claims"] = Encrypt(byteClaims)
	if claims["exp"] != nil {
		jwtClaims["exp"] = claims["exp"].(int64)
	}

	// create a signer for rsa 256
	t := jwt.NewWithClaims(jwt.GetSigningMethod("RS256"), jwtClaims)
	pub, err := jwt.ParseRSAPrivateKeyFromPEM(Config.Encryption.Private)
	if err != nil {
		return
	}
	token, err = t.SignedString(pub)
	if err != nil {
		return
	}
	return
}

//GenerateCookie ...
func GenerateCookie(jwtClaims jwt.MapClaims) (httpCookie *http.Cookie) {
	if jwtClaims == nil {
		httpCookie = &http.Cookie{
			Name: Config.Cookie, Value: "no-cookie", Path: "/",
			Expires: time.Now().Add(-(time.Hour * 24 * 30 * 12)), // set the expire time
			// SameSite: http.SameSiteNoneMode, Secure: true,
		}
		return
	}

	cookieExpires := time.Now().Add(time.Minute * 90) // set the expire time
	jwtClaims["exp"] = cookieExpires.Unix()

	if jwtToken, err := GenerateJWT(jwtClaims); err == nil {
		httpCookie = &http.Cookie{
			Name: Config.Cookie, Value: jwtToken, Expires: cookieExpires, Path: "/",
			// SameSite: http.SameSiteNoneMode, Secure: true,
		}
	} else {
		log.Println(err.Error())
	}
	return
}

//UpdateCookieTime ...
func UpdateCookieTime(httpRes http.ResponseWriter, httpReq *http.Request) {
	//Get Cookie if it exists update the time - thats all
	if monsterCookie, err := httpReq.Cookie(Config.Cookie); err == nil {
		if jwtClaims := ValidateJWT(monsterCookie.Value); jwtClaims != nil {
			jwtCookie := GenerateCookie(jwtClaims)
			http.SetCookie(httpRes, jwtCookie)
			// httpReq.AddCookie(jwtCookie)
		}
	}
}
