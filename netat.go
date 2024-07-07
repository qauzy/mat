package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/qauzy/mat/common/utils"
	"github.com/qauzy/mat/hub/executor"
	"github.com/qauzy/mat/log"
	"github.com/qauzy/mat/models"
	"github.com/qauzy/mat/x"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed all:tpl
var engine embed.FS

//	func StartCat(accessToken string) (err error) {
//		defer func() {
//			if r := recover(); r != nil {
//				fmt.Println("StartCat panicked:", r)
//				// 可以在这里执行一些恢复操作
//			}
//		}()
//		homeDir, err := utils.Home()
//		if err != nil {
//			return
//		}
//		constant.SetHomeDir(filepath.Join(homeDir, ".facat"))
//
//		if !fileExists(filepath.Join(homeDir, ".facat", "geoip.metadb")) {
//			// If aria2c binary doesn't exist, extract it.
//			if err = extractFile("engine/geoip.metadb", filepath.Join(homeDir, ".facat")); err != nil {
//				fmt.Printf("Initial extractFile  error: %s", err.Error())
//			}
//		}
//		loadRemoteConfig(accessToken)
//		return
//	}
func StopCat() {
	executor.Shutdown()
}

func WatchConfig(ctx context.Context, accessToken string) {

	go func() {
		tick := time.Tick(20 * time.Minute)
		for {
			select {
			case <-ctx.Done():
				logrus.Warnf("[config] stop config watching...")
				return
			case <-tick:
				loadRemoteConfig(accessToken)
			}
		}
	}()
	return
}

func loadRemoteConfig(accessToken string) (err error) {
	content, err := engine.ReadFile("engine/template.yml")
	if err != nil {
		fmt.Println("engine,err=", err)
		return
	}
	userConfigMap := make(map[string]interface{})
	err = yaml.Unmarshal(content, &userConfigMap)
	if err != nil {
		fmt.Println("yaml,err=", err)
		return
	}
	var proxies []interface{}
	var proxiesNameL []interface{}
	var proxiesNameH []interface{}
	proxiesInfo, err := GetProfile(accessToken)
	if err != nil {
		fmt.Println("GetProfile,err=", err)
		return
	}
	for _, p := range proxiesInfo.H {
		var proxy = make(map[string]interface{})
		yaml.Unmarshal([]byte(p.Info), proxy)
		tp, exit := proxy["type"].(string)
		//fix 没有type
		if exit == false {
			continue
		}
		// fix uuid无效问题
		if tp == "vmess" {
			uid, _ := proxy["uuid"].(string)
			if len(uid) != 36 {
				proxy["uuid"] = utils.NewUUIDV4()
			}
		}

		nt, exit := proxy["network"].(string)
		if exit && nt == "grpc" {
			tls, _ := proxy["tls"].(bool)
			if tls == false {
				continue
			}

		}

		proxies = append(proxies, proxy)

		proxiesNameH = append(proxiesNameH, p.Name)
	}
	for _, p := range proxiesInfo.L {

		var proxy = make(map[string]interface{})
		yaml.Unmarshal([]byte(p.Info), proxy)
		tp, exit := proxy["type"].(string)
		//fix 没有type
		if exit == false {
			continue
		}
		// fix uuid无效问题
		if tp == "vmess" {
			uid, _ := proxy["uuid"].(string)
			if len(uid) != 36 {
				proxy["uuid"] = utils.NewUUIDV4()
			}
		}

		nt, exit := proxy["network"].(string)
		if exit && nt == "grpc" {
			tls, _ := proxy["tls"].(bool)
			if tls == false {
				continue
			}

		}

		proxies = append(proxies, proxy)

		proxiesNameL = append(proxiesNameL, p.Name)
	}

	userConfigMap["proxies"] = proxies
	var proxyGroups = userConfigMap["proxy-groups"].([]interface{})

	var auto = make(map[string]any)
	auto["name"] = "♻️ 自动选择"
	auto["type"] = "load-balance"
	auto["url"] = "http://www.google.com/generate_204"
	auto["interval"] = 30
	auto["strategy"] = "round-robin"
	auto["proxies"] = proxiesNameH
	var hg = make(map[string]any)
	hg["name"] = "♻️ 负载均衡(huggingface)"
	hg["type"] = "load-balance"
	hg["url"] = "http://www.google.com/generate_204"
	hg["interval"] = 30
	hg["strategy"] = "round-robin"
	hg["proxies"] = proxiesNameL

	proxyGroups = append(proxyGroups, auto, hg)

	userConfigMap["proxy-groups"] = proxyGroups
	content, err = yaml.Marshal(userConfigMap)
	if err != nil {
		fmt.Println("yaml.Marshal,err=", err)
		return
	}
	log.Debugln("[config] config changed, reloading...")

	cfg, err := executor.ParseWithBytes(content)
	if err != nil {
		fmt.Println("executor.ParseWithPath err:", err)
		return
	}

	executor.ApplyConfig(cfg, true)

	log.Debugln("[config] netat config reload success...")
	return
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func extractFile(src, dstDir string) error {
	content, err := engine.ReadFile(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstPath := filepath.Join(dstDir, filepath.Base(src))
	if err := os.WriteFile(dstPath, content, 0644); err != nil {
		return err
	}

	return nil
}

type ProfileResult struct {
	Data    []byte `json:"data"`
	Message string `json:"message"`
	Success bool   `json:"success"`
}

func GetProfile(accessToken string) (info *models.ProxiesGroup, err error) {
	url := "https://fat.hiyai.cn/api/hf/profile"

	// 创建带有token的请求
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %v", err)
	}

	// 添加token到请求头
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("X-Version", x.VERSION)
	req.Header.Set("X-UUID", x.MachineData.PlatformUUID+"-"+x.MachineData.BoardSerialNumber+"-L")

	// 发送请求
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("发送请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 解析响应
	var profileResult ProfileResult
	err = json.NewDecoder(resp.Body).Decode(&profileResult)
	if err != nil {
		return nil, fmt.Errorf("JSON解码失败: %v", err)
	}

	// 检查搜索是否成功
	if !profileResult.Success {
		return nil, fmt.Errorf("请求失败")
	}
	//fmt.Println("data=", profileResult.Data)
	var decrypted []byte
	decrypted, err = utils.Decrypt([]byte(profileResult.Message[4:]), profileResult.Data)
	if err != nil {
		fmt.Println("解密失败:", err)
		return
	}

	err = json.Unmarshal(decrypted, &info)
	if err != nil {
		fmt.Println("Unmarshal失败,err=", err)
		return
	}
	return
}
