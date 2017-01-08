package p2p

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ripemd160"
	"math/big"
	"net"
	re "regexp"
	"time"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func Hexstr(data []byte) string {
	return hex.EncodeToString(data)
}

func ExternalIP() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	var ip net.IP
	for _, addr := range addrs {
		switch v := addr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		default:
			continue
		}
		if ip == nil || ip.IsLoopback() {
			continue
		}
		if ip = ip.To4(); ip == nil {
			continue
		}
		return ip, nil
	}
	return nil, errors.New("Could not find IP address")
}

const REGEX_IP = `(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})`

func ValidIP(host string) bool {
	if len(host) == 0 {
		return false
	}
	match, _ := re.MatchString(REGEX_IP, host)
	return match
}

func Timestamp() int64 {
	return time.Now().Unix()
}

var Hash = Sha1

func Sha1(data []byte) []byte {
	hash := sha1.New()
	hash.Write(data)
	return hash.Sum(nil)
}

func Ripemd160(data []byte) []byte {
	hash := ripemd160.New()
	hash.Write(data)
	return hash.Sum(nil)
}

const MAX_INT64 int64 = 9223372036854775807

func RandInt64(max int64) (int64, error) {
	bigmax := big.NewInt(max)
	bigint, err := rand.Int(rand.Reader, bigmax)
	if err != nil {
		return -1, err
	}
	i64 := bigint.Int64()
	if i64 < 0 {
		i64 *= -1
	}
	return i64, nil
}
