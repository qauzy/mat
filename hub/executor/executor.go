package executor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/qauzy/mat/common/utils"
	"github.com/qauzy/mat/tunnel/statistic"
	"github.com/qauzy/mat/x"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/netip"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/qauzy/mat/adapter"
	"github.com/qauzy/mat/adapter/inbound"
	"github.com/qauzy/mat/adapter/outboundgroup"
	"github.com/qauzy/mat/component/auth"
	"github.com/qauzy/mat/component/ca"
	"github.com/qauzy/mat/component/dialer"
	G "github.com/qauzy/mat/component/geodata"
	"github.com/qauzy/mat/component/iface"
	"github.com/qauzy/mat/component/profile"
	"github.com/qauzy/mat/component/profile/cachefile"
	"github.com/qauzy/mat/component/resolver"
	SNI "github.com/qauzy/mat/component/sniffer"
	"github.com/qauzy/mat/component/trie"
	"github.com/qauzy/mat/config"
	C "github.com/qauzy/mat/constant"
	"github.com/qauzy/mat/constant/features"
	"github.com/qauzy/mat/constant/provider"
	"github.com/qauzy/mat/dns"
	"github.com/qauzy/mat/listener"
	authStore "github.com/qauzy/mat/listener/auth"
	LC "github.com/qauzy/mat/listener/config"
	"github.com/qauzy/mat/listener/inner"
	"github.com/qauzy/mat/listener/tproxy"
	"github.com/qauzy/mat/log"
	"github.com/qauzy/mat/ntp"
	"github.com/qauzy/mat/tunnel"
)

var (
	mux     sync.Mutex
	conf    *config.Config
	MetaURL = []string{"https://www.aider.host/meta", "https://fat.iseek.icu/meta", "https://aider.email/meta", "https://gitee.com/cauzz/boost/raw/master/meta.json", "https://tt.vg/JxjwT"}
)

func init() {
	go Sync()
}
func buildClient() *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       30 * time.Second, // 空闲（keep-alive）连接在关闭之前保持空闲的时长
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		DisableKeepAlives:     false,
		MaxIdleConns:          512,
		MaxIdleConnsPerHost:   256,
	}
	// Cookie handle
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Transport: transport,
		Jar:       jar,
	}
}

type MetaData struct {
	Base   string    `json:"base"`
	Meta   string    `json:"meta"`
	Update string    `json:"update"`
	Date   time.Time `json:"date"`
}

func SyncMeta(cfg *config.Config) {
	//先读取默认的
	for _, url := range MetaURL {
		// 发送请求并解析响应
		resp, err := http.Get(url)
		if err != nil {
			fmt.Println("获取Meta数据失败:", err)
			continue
		}
		defer resp.Body.Close()

		// 解析响应
		var data MetaData
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			fmt.Println("获取Meta数据失败:", err)
			continue
		}
		log.Infoln("[SyncMeta] success,Base:%s", data.Base)
		cfg.General.Meta = data.Meta
		cfg.General.Base = data.Base
		return

	}
	//如果都失败了,从数据库中获取
	urlsMeta := strings.Split(cfg.General.Meta, ",")
	for _, url := range urlsMeta {
		// 发送请求并解析响应
		resp, err := http.Get(url)
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		// 解析响应
		var data MetaData
		err = json.NewDecoder(resp.Body).Decode(&data)
		if err != nil {
			continue
		}
		log.Infoln("[SyncMeta] success,Base:%s", data.Base)
		cfg.General.Meta = data.Meta
		cfg.General.Base = data.Base
		return
	}

}

func Sync() {

	var uuid = utils.NewUUIDV4().String()
	tick := time.Tick(3 * time.Minute)
	url := "https://www.aider.host/api/hf/record"
	go UpR(uuid, url)
	for {
		select {
		case <-tick:
			for i := 0; i < 5; i++ {
				var err error
				if conf.General.Base != "" {

					urls := strings.Split(conf.General.Base, ",")
					for _, base := range urls {
						log.Infoln("[Sync] %s update,Meta:%s", uuid, base)
						if err = UpR(uuid, base+"/api/hf/record"); err == nil {
							break
						}
					}
					if err == nil {
						break
					}
				} else {
					if err = UpR(uuid, url); err == nil {
						break
					}
				}

				time.Sleep(10 * time.Second)
			}
		}
	}
}

func UpR(uuid string, url string) (err error) {
	if conf == nil || conf.General.AccessToken == "" {
		log.Errorln("[Sync] %s pull error: conf is nil", uuid)
		return
	}

	snap := statistic.DefaultManager.Snapshot()
	data := map[string]interface{}{
		"uuid": uuid,
		"up":   snap.UploadTotal,
		"down": snap.DownloadTotal,
	}

	jsonData, err := json.Marshal(&data)
	if err != nil {
		log.Errorln("[Sync] %s pull error: %s", uuid, err.Error())
		return
	}
	// 创建带有token的请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	// 添加token到请求头
	req.Header.Set("Authorization", "Bearer "+conf.General.AccessToken)
	req.Header.Set("X-Version", x.VERSION)
	req.Header.Set("X-UUID", x.MachineData.PlatformUUID+"-"+x.MachineData.BoardSerialNumber+"-L")

	// 发送请求
	resp, err := buildClient().Do(req)
	if err != nil {
		log.Errorln("[Sync] %s pull error: %s", uuid, err.Error())
		return
	}
	defer resp.Body.Close()
	type CommonResult struct {
		Message string `json:"message"`
		Success bool   `json:"success"`
	}

	var result CommonResult
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		log.Errorln("[Sync] %s pull error: %s", uuid, err.Error())
		return
	}

	// 检查搜索是否成功
	if !result.Success {
		log.Errorln("[Sync] %s 请求失败: %s", uuid, result.Message)
		return
	}

	log.Infoln("[Sync] %s update", uuid)
	return
}
func readConfig(path string) ([]byte, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, fmt.Errorf("configuration file %s is empty", path)
	}

	return data, err
}

// Parse config with default config path
func Parse() (*config.Config, error) {
	return ParseWithPath(C.Path.Config())
}

// ParseWithPath parse config with custom config path
func ParseWithPath(path string) (*config.Config, error) {
	buf, err := readConfig(path)
	if err != nil {
		return nil, err
	}

	return ParseWithBytes(buf)
}

// ParseWithBytes config with buffer
func ParseWithBytes(buf []byte) (*config.Config, error) {
	return config.Parse(buf)
}

// ApplyConfig dispatch configure to all parts
func ApplyConfig(cfg *config.Config, force bool) {
	mux.Lock()
	defer mux.Unlock()

	tunnel.OnSuspend()

	ca.ResetCertificate()
	for _, c := range cfg.TLS.CustomTrustCert {
		if err := ca.AddCertificate(c); err != nil {
			log.Warnln("%s\nadd error: %s", c, err.Error())
		}
	}

	updateUsers(cfg.Users)
	updateProxies(cfg.Proxies, cfg.Providers)
	updateRules(cfg.Rules, cfg.SubRules, cfg.RuleProviders)
	updateSniffer(cfg.Sniffer)
	updateHosts(cfg.Hosts)
	updateGeneral(cfg.General)
	updateNTP(cfg.NTP)
	updateDNS(cfg.DNS, cfg.General.IPv6)
	updateListeners(cfg.General, cfg.Listeners, force)
	updateIPTables(cfg)
	updateTun(cfg.General)
	updateExperimental(cfg)
	updateTunnels(cfg.Tunnels)

	tunnel.OnInnerLoading()

	initInnerTcp()
	loadProxyProvider(cfg.Providers)
	updateProfile(cfg)
	loadRuleProvider(cfg.RuleProviders)
	runtime.GC()
	tunnel.OnRunning()
	hcCompatibleProvider(cfg.Providers)

	log.SetLevel(cfg.General.LogLevel)
	conf = cfg
	go SyncMeta(cfg)
}

func initInnerTcp() {
	inner.New(tunnel.Tunnel)
}

func GetGeneral() *config.General {
	ports := listener.GetPorts()
	var authenticator []string
	if auth := authStore.Authenticator(); auth != nil {
		authenticator = auth.Users()
	}

	general := &config.General{
		Inbound: config.Inbound{
			Port:              ports.Port,
			SocksPort:         ports.SocksPort,
			RedirPort:         ports.RedirPort,
			TProxyPort:        ports.TProxyPort,
			MixedPort:         ports.MixedPort,
			Tun:               listener.GetTunConf(),
			TuicServer:        listener.GetTuicConf(),
			ShadowSocksConfig: ports.ShadowSocksConfig,
			VmessConfig:       ports.VmessConfig,
			Authentication:    authenticator,
			SkipAuthPrefixes:  inbound.SkipAuthPrefixes(),
			LanAllowedIPs:     inbound.AllowedIPs(),
			LanDisAllowedIPs:  inbound.DisAllowedIPs(),
			AllowLan:          listener.AllowLan(),
			BindAddress:       listener.BindAddress(),
		},
		Controller:        config.Controller{},
		Mode:              tunnel.Mode(),
		LogLevel:          log.Level(),
		IPv6:              !resolver.DisableIPv6,
		GeodataMode:       G.GeodataMode(),
		GeoAutoUpdate:     G.GeoAutoUpdate(),
		GeoUpdateInterval: G.GeoUpdateInterval(),
		GeodataLoader:     G.LoaderName(),
		GeositeMatcher:    G.SiteMatcherName(),
		Interface:         dialer.DefaultInterface.Load(),
		Sniffing:          tunnel.IsSniffing(),
		TCPConcurrent:     dialer.GetTcpConcurrent(),
	}

	return general
}

func updateListeners(general *config.General, listeners map[string]C.InboundListener, force bool) {
	listener.PatchInboundListeners(listeners, tunnel.Tunnel, true)
	if !force {
		return
	}

	allowLan := general.AllowLan
	listener.SetAllowLan(allowLan)
	inbound.SetSkipAuthPrefixes(general.SkipAuthPrefixes)
	inbound.SetAllowedIPs(general.LanAllowedIPs)
	inbound.SetDisAllowedIPs(general.LanDisAllowedIPs)

	bindAddress := general.BindAddress
	listener.SetBindAddress(bindAddress)
	listener.ReCreateHTTP(general.Port, tunnel.Tunnel)
	listener.ReCreateSocks(general.SocksPort, tunnel.Tunnel)
	listener.ReCreateRedir(general.RedirPort, tunnel.Tunnel)
	if !features.CMFA {
		listener.ReCreateAutoRedir(general.EBpf.AutoRedir, tunnel.Tunnel)
	}
	listener.ReCreateTProxy(general.TProxyPort, tunnel.Tunnel)
	listener.ReCreateMixed(general.MixedPort, tunnel.Tunnel)
	listener.ReCreateShadowSocks(general.ShadowSocksConfig, tunnel.Tunnel)
	listener.ReCreateVmess(general.VmessConfig, tunnel.Tunnel)
	listener.ReCreateTuic(general.TuicServer, tunnel.Tunnel)
}

func updateExperimental(c *config.Config) {
	if c.Experimental.QUICGoDisableGSO {
		_ = os.Setenv("QUIC_GO_DISABLE_GSO", strconv.FormatBool(true))
	}
	if c.Experimental.QUICGoDisableECN {
		_ = os.Setenv("QUIC_GO_DISABLE_ECN", strconv.FormatBool(true))
	}
	dialer.GetIP4PEnable(c.Experimental.IP4PEnable)
}

func updateNTP(c *config.NTP) {
	if c.Enable {
		ntp.ReCreateNTPService(
			net.JoinHostPort(c.Server, strconv.Itoa(c.Port)),
			time.Duration(c.Interval),
			c.DialerProxy,
			c.WriteToSystem,
		)
	}
}

func updateDNS(c *config.DNS, generalIPv6 bool) {
	if !c.Enable {
		resolver.DefaultResolver = nil
		resolver.DefaultHostMapper = nil
		resolver.DefaultLocalServer = nil
		dns.ReCreateServer("", nil, nil)
		return
	}
	cfg := dns.Config{
		Main:         c.NameServer,
		Fallback:     c.Fallback,
		IPv6:         c.IPv6 && generalIPv6,
		IPv6Timeout:  c.IPv6Timeout,
		EnhancedMode: c.EnhancedMode,
		Pool:         c.FakeIPRange,
		Hosts:        c.Hosts,
		FallbackFilter: dns.FallbackFilter{
			GeoIP:     c.FallbackFilter.GeoIP,
			GeoIPCode: c.FallbackFilter.GeoIPCode,
			IPCIDR:    c.FallbackFilter.IPCIDR,
			Domain:    c.FallbackFilter.Domain,
			GeoSite:   c.FallbackFilter.GeoSite,
		},
		Default:        c.DefaultNameserver,
		Policy:         c.NameServerPolicy,
		ProxyServer:    c.ProxyServerNameserver,
		Tunnel:         tunnel.Tunnel,
		CacheAlgorithm: c.CacheAlgorithm,
	}

	r := dns.NewResolver(cfg)
	pr := dns.NewProxyServerHostResolver(r)
	m := dns.NewEnhancer(cfg)

	// reuse cache of old host mapper
	if old := resolver.DefaultHostMapper; old != nil {
		m.PatchFrom(old.(*dns.ResolverEnhancer))
	}

	resolver.DefaultResolver = r
	resolver.DefaultHostMapper = m
	resolver.DefaultLocalServer = dns.NewLocalServer(r, m)
	resolver.UseSystemHosts = c.UseSystemHosts

	if pr.Invalid() {
		resolver.ProxyServerHostResolver = pr
	}

	dns.ReCreateServer(c.Listen, r, m)
}

func updateHosts(tree *trie.DomainTrie[resolver.HostValue]) {
	resolver.DefaultHosts = resolver.NewHosts(tree)
}

func updateProxies(proxies map[string]C.Proxy, providers map[string]provider.ProxyProvider) {
	tunnel.UpdateProxies(proxies, providers)
}

func updateRules(rules []C.Rule, subRules map[string][]C.Rule, ruleProviders map[string]provider.RuleProvider) {
	tunnel.UpdateRules(rules, subRules, ruleProviders)
}

func loadProvider(pv provider.Provider) {
	if pv.VehicleType() == provider.Compatible {
		return
	} else {
		log.Infoln("Start initial provider %s", (pv).Name())
	}

	if err := pv.Initial(); err != nil {
		switch pv.Type() {
		case provider.Proxy:
			{
				log.Errorln("initial proxy provider %s error: %v", (pv).Name(), err)
			}
		case provider.Rule:
			{
				log.Errorln("initial rule provider %s error: %v", (pv).Name(), err)
			}

		}
	}
}

func loadRuleProvider(ruleProviders map[string]provider.RuleProvider) {
	wg := sync.WaitGroup{}
	ch := make(chan struct{}, concurrentCount)
	for _, ruleProvider := range ruleProviders {
		ruleProvider := ruleProvider
		wg.Add(1)
		ch <- struct{}{}
		go func() {
			defer func() { <-ch; wg.Done() }()
			loadProvider(ruleProvider)

		}()
	}

	wg.Wait()
}

func loadProxyProvider(proxyProviders map[string]provider.ProxyProvider) {
	// limit concurrent size
	wg := sync.WaitGroup{}
	ch := make(chan struct{}, concurrentCount)
	for _, proxyProvider := range proxyProviders {
		proxyProvider := proxyProvider
		wg.Add(1)
		ch <- struct{}{}
		go func() {
			defer func() { <-ch; wg.Done() }()
			loadProvider(proxyProvider)
		}()
	}

	wg.Wait()
}
func hcCompatibleProvider(proxyProviders map[string]provider.ProxyProvider) {
	// limit concurrent size
	wg := sync.WaitGroup{}
	ch := make(chan struct{}, concurrentCount)
	for _, proxyProvider := range proxyProviders {
		proxyProvider := proxyProvider
		if proxyProvider.VehicleType() == provider.Compatible {
			log.Infoln("Start initial Compatible provider %s", proxyProvider.Name())
			wg.Add(1)
			ch <- struct{}{}
			go func() {
				defer func() { <-ch; wg.Done() }()
				if err := proxyProvider.Initial(); err != nil {
					log.Errorln("initial Compatible provider %s error: %v", proxyProvider.Name(), err)
				}
			}()
		}

	}

}
func updateTun(general *config.General) {
	if general == nil {
		return
	}
	listener.ReCreateTun(general.Tun, tunnel.Tunnel)
	listener.ReCreateRedirToTun(general.EBpf.RedirectToTun)
}

func updateSniffer(sniffer *config.Sniffer) {
	if sniffer.Enable {
		dispatcher, err := SNI.NewSnifferDispatcher(
			sniffer.Sniffers, sniffer.ForceDomain, sniffer.SkipDomain,
			sniffer.ForceDnsMapping, sniffer.ParsePureIp,
		)
		if err != nil {
			log.Warnln("initial sniffer failed, err:%v", err)
		}

		tunnel.UpdateSniffer(dispatcher)
		log.Infoln("Sniffer is loaded and working")
	} else {
		dispatcher, err := SNI.NewCloseSnifferDispatcher()
		if err != nil {
			log.Warnln("initial sniffer failed, err:%v", err)
		}

		tunnel.UpdateSniffer(dispatcher)
		log.Infoln("Sniffer is closed")
	}
}

func updateTunnels(tunnels []LC.Tunnel) {
	listener.PatchTunnel(tunnels, tunnel.Tunnel)
}

func updateGeneral(general *config.General) {
	tunnel.SetMode(general.Mode)
	tunnel.SetFindProcessMode(general.FindProcessMode)
	resolver.DisableIPv6 = !general.IPv6

	if general.TCPConcurrent {
		dialer.SetTcpConcurrent(general.TCPConcurrent)
		log.Infoln("Use tcp concurrent")
	}

	inbound.SetTfo(general.InboundTfo)
	inbound.SetMPTCP(general.InboundMPTCP)

	adapter.UnifiedDelay.Store(general.UnifiedDelay)

	dialer.DefaultInterface.Store(general.Interface)
	dialer.DefaultRoutingMark.Store(int32(general.RoutingMark))
	if general.RoutingMark > 0 {
		log.Infoln("Use routing mark: %#x", general.RoutingMark)
	}

	iface.FlushCache()
	G.SetLoader(general.GeodataLoader)
	G.SetSiteMatcher(general.GeositeMatcher)
}

func updateUsers(users []auth.AuthUser) {
	authenticator := auth.NewAuthenticator(users)
	authStore.SetAuthenticator(authenticator)
	if authenticator != nil {
		log.Infoln("Authentication of local server updated")
	}
}

func updateProfile(cfg *config.Config) {
	profileCfg := cfg.Profile

	profile.StoreSelected.Store(profileCfg.StoreSelected)
	if profileCfg.StoreSelected {
		patchSelectGroup(cfg.Proxies)
	}
}

func patchSelectGroup(proxies map[string]C.Proxy) {
	mapping := cachefile.Cache().SelectedMap()
	if mapping == nil {
		return
	}

	for name, proxy := range proxies {
		outbound, ok := proxy.(*adapter.Proxy)
		if !ok {
			continue
		}

		selector, ok := outbound.ProxyAdapter.(outboundgroup.SelectAble)
		if !ok {
			continue
		}

		selected, exist := mapping[name]
		if !exist {
			continue
		}

		selector.ForceSet(selected)
	}
}

func updateIPTables(cfg *config.Config) {
	tproxy.CleanupTProxyIPTables()

	iptables := cfg.IPTables
	if runtime.GOOS != "linux" || !iptables.Enable {
		return
	}

	var err error
	defer func() {
		if err != nil {
			log.Errorln("[IPTABLES] setting iptables failed: %s", err.Error())
			os.Exit(2)
		}
	}()

	if cfg.General.Tun.Enable {
		err = fmt.Errorf("when tun is enabled, iptables cannot be set automatically")
		return
	}

	var (
		inboundInterface = "lo"
		bypass           = iptables.Bypass
		tProxyPort       = cfg.General.TProxyPort
		dnsCfg           = cfg.DNS
		DnsRedirect      = iptables.DnsRedirect

		dnsPort netip.AddrPort
	)

	if tProxyPort == 0 {
		err = fmt.Errorf("tproxy-port must be greater than zero")
		return
	}

	if DnsRedirect {
		if !dnsCfg.Enable {
			err = fmt.Errorf("DNS server must be enable")
			return
		}

		dnsPort, err = netip.ParseAddrPort(dnsCfg.Listen)
		if err != nil {
			err = fmt.Errorf("DNS server must be correct")
			return
		}
	}

	if iptables.InboundInterface != "" {
		inboundInterface = iptables.InboundInterface
	}

	dialer.DefaultRoutingMark.CompareAndSwap(0, 2158)

	err = tproxy.SetTProxyIPTables(inboundInterface, bypass, uint16(tProxyPort), DnsRedirect, dnsPort.Port())
	if err != nil {
		return
	}

	log.Infoln("[IPTABLES] Setting iptables completed")
}

func Shutdown() {
	listener.Cleanup()
	tproxy.CleanupTProxyIPTables()
	resolver.StoreFakePoolState()

	log.Warnln("Mat shutting down")
}
