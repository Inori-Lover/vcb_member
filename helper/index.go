package helper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	mathRand "math/rand"
	"strconv"
	"time"
	"vcb_member/conf"
	"vcb_member/models"

	"github.com/btnguyen2k/olaf"
	argon "github.com/dwin/goArgonPass"
	"github.com/pascaldekloe/jwt"
	"golang.org/x/crypto/blake2b"
)

// timeStart jwt时间偏移
// new Number( new Date('Mon Dec 01 2010 00:00:00 GMT+0800') )
const timeStart = 1291132800000
const jwtIssuer = "vcb-member"
const jwtExpires = 30 * time.Minute
const jwtRefreshExpires = 7 * 24 * time.Hour

var idGenerator *olaf.Olaf
var signKey = []byte(conf.Main.Jwt.Mac)
var encryptKey = []byte(conf.Main.Jwt.Encryption)

// ErrorExpired jwt过期
const ErrorExpired = "Expired"

// ErrorInvalid jwt无效
const ErrorInvalid = "Invalid"

func init() {
	idGenerator = olaf.NewOlafWithEpoch(1, timeStart)
}

// GenID 获取一个雪花ID
func GenID() string {
	return idGenerator.Id64Ascii()
}

// GenToken 获取一个jwt
func GenToken(uid string) (string, error) {
	var claims jwt.Claims
	now := time.Now().Round(time.Second)
	claims.Issuer = jwtIssuer
	claims.Issued = jwt.NewNumericTime(now)
	claims.Expires = jwt.NewNumericTime(now.Add(jwtExpires))
	claims.Subject = uid

	token, err := claims.HMACSign(jwt.HS256, signKey)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// CheckToken 检查jwt
func CheckToken(tokenString []byte) (string, error) {
	claims, err := jwt.HMACCheck(tokenString, signKey)
	if err != nil {
		return "", err
	}

	if !claims.Valid((time.Now())) {
		return "", errors.New(ErrorExpired)
	}

	return claims.Subject, nil
}

// GenRefreshToken 获取一个 refreshjwt
func GenRefreshToken(uid string) (string, error) {
	var claims jwt.Claims
	now := time.Now().Round(time.Second)
	jwtID := GenID()

	// 记录到用户的token字段，refresh的时候读取验证
	user := models.User{JwtID: jwtID}
	_, err := models.GetDBHelper().Table("user").Where("id = ?", uid).Update(&user)
	if err != nil {
		return "", err
	}

	claims.ID = jwtID
	claims.Issuer = jwtIssuer
	claims.Issued = jwt.NewNumericTime(now)
	claims.Expires = jwt.NewNumericTime(now.Add(jwtRefreshExpires))
	claims.Subject = uid

	token, err := claims.HMACSign(jwt.HS256, signKey)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// ReGenRefreshToken 重签 refreshjwt
func ReGenRefreshToken(originToken []byte) (string, error) {
	claims, err := jwt.HMACCheck(originToken, signKey)
	if err != nil {
		return "", err
	}
	now := time.Now().Round(time.Second)

	claims.Issued = jwt.NewNumericTime(now)
	claims.Expires = jwt.NewNumericTime(now.Add(jwtRefreshExpires))

	token, err := claims.HMACSign(jwt.HS256, signKey)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// CheckRefreshToken 检查jwt
func CheckRefreshToken(token []byte) (string, error) {
	claims, err := jwt.HMACCheck(token, signKey)
	if err != nil {
		return "", err
	}

	uid := claims.Subject

	if !claims.Valid((time.Now())) {
		return "", errors.New(ErrorExpired)
	}

	// 查询用户的 tokenID 字段
	tokenID := claims.ID
	var user models.User
	hasValue, err := models.GetDBHelper().Table("user").Where("id = ? and jwt_id = ?", uid, tokenID).Get(&user)
	if err != nil {
		return "", err
	}
	if !hasValue {
		return "", errors.New(ErrorInvalid)
	}

	return uid, nil
}

// CalcPassHash 获取一个安全的密码Hash
func CalcPassHash(pass string) (string, error) {
	return argon.Hash(pass)
}

// CheckPassHash 校验密码
func CheckPassHash(pass string, hash string) bool {
	err := argon.Verify(pass, hash)
	if err != nil {
		return false
	}

	return true
}

// GenIVByte 产生随机IV
func GenIVByte() ([]byte, error) {
	nonce := make([]byte, 12)
	io.ReadFull(rand.Reader, nonce)
	_, err := io.ReadFull(rand.Reader, nonce)

	return nonce, err
}

// GenRandPass 产生一个8位随机数字密码
func GenRandPass() string {
	var newPass string
	for i := 0; i < 8; i++ {
		newPass += strconv.Itoa(mathRand.Intn(9))
	}

	return newPass
}

// GenHashtext 加密给定字符串
func GenHashtext(plaintext string) string {
	plainByte := []byte(plaintext)
	hash := blake2b.Sum256(plainByte)
	return base64.URLEncoding.EncodeToString(hash[:])
}

// Ciphertext 加密结果
type Ciphertext struct {
	Iv         string
	Ciphertext string
}

// GenCiphertext 加密给定字符串
func GenCiphertext(plaintext string) (Ciphertext, error) {
	plainByte := []byte(plaintext)
	var result Ciphertext

	iv, err := GenIVByte()
	if err != nil {
		panic(err)
	}

	result.Iv = base64.URLEncoding.EncodeToString(iv)

	block, err := aes.NewCipher(encryptKey)
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		panic(err.Error())
	}

	result.Ciphertext = base64.URLEncoding.EncodeToString(aesgcm.Seal(nil, iv, plainByte, nil))

	return result, err
}
