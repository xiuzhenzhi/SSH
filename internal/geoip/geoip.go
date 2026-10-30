package geoip

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"sshguard/internal/ipaddress"
	"strings"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const DatabaseURL = "https://raw.githubusercontent.com/Loyalsoldier/geoip/release/GeoLite2-Country.mmdb"
const MaxDatabaseBytes = 32 << 20

type Config struct {
	Provider string `json:"provider" binding:"required,oneof=none maxmind ip-api geojs ipinfo"`
	Token    string `json:"token" binding:"max=512"`
}

type Location struct {
	Code string `json:"country_code"`
	Name string `json:"country_name"`
}

func NewClient() *http.Client {
	return &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

func normalize(location Location) (Location, error) {
	location.Code = strings.ToUpper(strings.TrimSpace(location.Code))
	if len(location.Code) != 2 || location.Code[0] < 'A' || location.Code[0] > 'Z' || location.Code[1] < 'A' || location.Code[1] > 'Z' || location.Code == "ZZ" || location.Code == "XX" {
		return Location{}, errors.New("未查询到国家或地区")
	}
	if location.Name == "" {
		location.Name = location.Code
	}
	return location, nil
}

func Lookup(ctx context.Context, client *http.Client, cfg Config, address string) (Location, error) {
	if !ipaddress.Public(address) {
		return Location{}, errors.New("请输入公网 IPv4 或 IPv6 地址")
	}
	address = netip.MustParseAddr(address).Unmap().String()
	var endpoint string
	switch cfg.Provider {
	case "geojs":
		endpoint = "https://get.geojs.io/v1/ip/geo/" + address + ".json"
	case "ipinfo":
		endpoint = "https://ipinfo.io/" + address + "/json"
		if cfg.Token != "" {
			endpoint = "https://api.ipinfo.io/lite/" + address
		}
	case "ip-api":
		endpoint = "http://ip-api.com/json/" + address + "?fields=status,message,country,countryCode"
		if cfg.Token != "" {
			endpoint = "https://pro.ip-api.com/json/" + address + "?fields=status,message,country,countryCode&key=" + url.QueryEscape(cfg.Token)
		}
	default:
		return Location{}, errors.New("请先选择在线 GeoIP 数据提供商")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Location{}, err
	}
	req.Header.Set("Accept", "application/json")
	if cfg.Provider == "ipinfo" && cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return Location{}, errors.New("GeoIP 服务连接失败，请稍后重试")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Location{}, fmt.Errorf("GeoIP 服务返回 HTTP %d，请检查配额或凭证", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
	if err != nil || len(raw) > 64<<10 {
		return Location{}, errors.New("GeoIP 服务响应过大或读取失败")
	}
	var value struct {
		Code    string `json:"country_code"`
		APICode string `json:"countryCode"`
		Country string `json:"country"`
		Status  string `json:"status"`
	}
	if json.Unmarshal(raw, &value) != nil {
		return Location{}, errors.New("GeoIP 服务响应格式无效")
	}
	location := Location{Code: value.Code, Name: value.Country}
	if cfg.Provider == "ip-api" {
		if value.Status != "success" {
			return Location{}, errors.New("GeoIP 服务未能解析此 IP")
		}
		location.Code = value.APICode
	} else if cfg.Provider == "ipinfo" && cfg.Token == "" {
		location.Code = value.Country
	}
	return normalize(location)
}

func LookupDatabase(reader *maxminddb.Reader, address string) (Location, error) {
	if !ipaddress.Public(address) {
		return Location{}, errors.New("请输入公网 IPv4 或 IPv6 地址")
	}
	var record struct {
		Country struct {
			Code  string            `maxminddb:"iso_code"`
			Names map[string]string `maxminddb:"names"`
		} `maxminddb:"country"`
	}
	if err := reader.Lookup(netip.MustParseAddr(address).Unmap()).Decode(&record); err != nil {
		return Location{}, err
	}
	name := record.Country.Names["zh-CN"]
	if name == "" {
		name = record.Country.Names["en"]
	}
	return normalize(Location{Code: record.Country.Code, Name: name})
}

// Validate before replacing the saved database so failed updates preserve it.
func ValidateDatabase(raw []byte) (*maxminddb.Reader, error) {
	reader, err := maxminddb.OpenBytes(raw)
	if err != nil {
		return nil, errors.New("下载内容不是有效的 MaxMind 数据库")
	}
	if !strings.Contains(reader.Metadata.DatabaseType, "Country") && !strings.Contains(reader.Metadata.DatabaseType, "City") {
		reader.Close()
		return nil, errors.New("数据库不包含国家或地区信息")
	}
	if err := reader.Verify(); err != nil {
		reader.Close()
		return nil, errors.New("MaxMind 数据库完整性校验失败")
	}
	return reader, nil
}
