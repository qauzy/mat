package cloudflare

import (
	"fmt"
	"github.com/qauzy/tpat/models"
	"net"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	tcpConnectTimeout = time.Second * 1
	maxRoutine        = 1000
	defaultRoutines   = 200
	defaultPort       = 443
	defaultPingTimes  = 4
)

var (
	Routines      = defaultRoutines
	TCPPort   int = defaultPort
	PingTimes int = defaultPingTimes

	IpTotal atomic.Int64
	IpDone  atomic.Int64
)

type Ping struct {
	wg      *sync.WaitGroup
	m       *sync.Mutex
	csv     models.PingDelaySet
	ips     []*net.IPAddr
	control chan bool
}

var BetterIP string
var ReCheck = make(chan bool)

var isChecking = false
var IsWatching = false

func CheckCF() {
	if isChecking {
		return
	}
	isChecking = true
	defer func() {
		isChecking = false
	}()
	// 开始延迟测速 + 过滤延迟/丢包
	InitRandSeed() // 置随机数种子

	// 开始延迟测速 + 过滤延迟/丢包
	pingData := NewPing().Run().FilterDelay().FilterLossRate()
	fmt.Println("=====================================")
	for _, p := range pingData {
		fmt.Printf("IP:%s delay:%v\n", p.IP, p.Delay)
	}
	//// 开始下载测速
	//speedData := TestDownloadSpeed(pingData)
	//if len(speedData) > 0 {
	//	for _, p := range speedData {
	//		fmt.Printf("IP:%s delay:%v\n", p.IP, p.Delay)
	//	}
	//	BetterIP = speedData[0].IP.String()
	//}
}

func checkPingDefault() {
	if Routines <= 0 {
		Routines = defaultRoutines
	}
	if TCPPort <= 0 || TCPPort >= 65535 {
		TCPPort = defaultPort
	}
	if PingTimes <= 0 {
		PingTimes = defaultPingTimes
	}
}

func NewPing() *Ping {
	checkPingDefault()
	ips := loadIPRanges()
	IpTotal.Store(int64(len(ips)))
	IpDone.Store(0)
	return &Ping{
		wg:      &sync.WaitGroup{},
		m:       &sync.Mutex{},
		csv:     make(models.PingDelaySet, 0),
		ips:     ips,
		control: make(chan bool, Routines),
	}
}

func (p *Ping) Run() models.PingDelaySet {
	if len(p.ips) == 0 {
		return p.csv
	}
	if Httping {
		fmt.Printf("开始延迟测速（模式：HTTP, 端口：%d, 范围：%v ~ %v ms, 丢包：%.2f)\n", TCPPort, models.InputMinDelay.Milliseconds(), models.InputMaxDelay.Milliseconds(), models.InputMaxLossRate)
	} else {
		fmt.Printf("开始延迟测速（模式：TCP, 端口：%d, 范围：%v ~ %v ms, 丢包：%.2f)\n", TCPPort, models.InputMinDelay.Milliseconds(), models.InputMaxDelay.Milliseconds(), models.InputMaxLossRate)
	}
	for _, ip := range p.ips {
		p.wg.Add(1)
		p.control <- false
		go p.start(ip)
	}
	p.wg.Wait()
	//p.bar.Done()
	sort.Sort(p.csv)
	return p.csv
}

func (p *Ping) start(ip *net.IPAddr) {
	defer p.wg.Done()
	p.tcpingHandler(ip)
	<-p.control
}

// bool connectionSucceed float32 time
func (p *Ping) tcping(ip *net.IPAddr) (bool, time.Duration) {
	startTime := time.Now()
	var fullAddress string
	if isIPv4(ip.String()) {
		fullAddress = fmt.Sprintf("%s:%d", ip.String(), TCPPort)
	} else {
		fullAddress = fmt.Sprintf("[%s]:%d", ip.String(), TCPPort)
	}
	conn, err := net.DialTimeout("tcp", fullAddress, tcpConnectTimeout)
	if err != nil {
		return false, 0
	}
	defer conn.Close()
	duration := time.Since(startTime)
	return true, duration
}

// pingReceived pingTotalTime
func (p *Ping) checkConnection(ip *net.IPAddr) (recv int, totalDelay time.Duration) {
	if Httping {
		recv, totalDelay = p.httping(ip)
		return
	}
	for i := 0; i < PingTimes; i++ {
		if ok, delay := p.tcping(ip); ok {
			recv++
			totalDelay += delay
		}
	}
	return
}

func (p *Ping) appendIPData(data *models.PingData) {
	p.m.Lock()
	defer p.m.Unlock()
	p.csv = append(p.csv, models.CloudflareIPData{
		PingData: data,
	})
}

// handle tcping
func (p *Ping) tcpingHandler(ip *net.IPAddr) {
	recv, totalDlay := p.checkConnection(ip)
	nowAble := len(p.csv)
	if recv != 0 {
		nowAble++
	}
	IpDone.Add(1)
	if recv == 0 {
		return
	}
	data := &models.PingData{
		IP:       ip,
		Sended:   PingTimes,
		Received: recv,
		Delay:    totalDlay / time.Duration(recv),
	}
	fmt.Printf("IP:%s delay:%v\n", ip, totalDlay/time.Duration(recv))
	p.appendIPData(data)
}
